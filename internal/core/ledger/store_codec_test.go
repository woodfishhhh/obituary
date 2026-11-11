package ledger

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/internal/fileio"
)

// Measure the whole file-backed transaction, including production fsyncs.
func BenchmarkStoreLedger(b *testing.B) {
	previous := fileio.SyncFile
	fileio.SyncFile = (*os.File).Sync
	b.Cleanup(func() { fileio.SyncFile = previous })
	for _, operation := range []string{"load", "load-hot", "save", "update-single", "update-joint"} {
		b.Run(operation, func(b *testing.B) {
			store := Store{Home: b.TempDir()}
			state := benchmarkLedger()
			if err := store.save(state); err != nil {
				b.Fatal(err)
			}
			info, err := os.Stat(store.statePath())
			if err != nil {
				b.Fatal(err)
			}
			committed, _, err := store.load()
			if err != nil {
				b.Fatal(err)
			}
			snapshot, err := os.Stat(filepath.Join(store.Home, committed.Graveyard.File))
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.SetBytes(info.Size())
			for b.Loop() {
				switch operation {
				case "load":
					_, _, err = store.load()
				case "load-hot":
					_, _, err = store.loadHot()
				case "save":
					err = store.save(state)
				case "update-single":
					err = store.UpdateHot(func(st *State) error { st.Items[0].Rev++; return nil })
				case "update-joint":
					err = store.UpdateGraveyard(func(st *State) error { st.Items[0].Rev++; st.Burials[0].Size++; return nil })
				}
				if err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(info.Size()), "ledger-B")
			b.ReportMetric(float64(snapshot.Size()), "snapshot-B")
		})
	}
}

func benchmarkLedger() State {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	state := State{Version: stateVersion, NextID: 1466, DataID: "synthetic-benchmark"}
	for i := range 1300 {
		group := i % 20
		if i%10 == 0 {
			group = i + 20
		}
		parent := fmt.Sprintf(`C:\agent\workspace\example-project\scratch\group-%04d`, group)
		var parents []BurialParent
		for level, path := 0, parent; level < 5; level, path = level+1, filepath.Dir(path) {
			parents = append(parents, BurialParent{Path: path, Identity: *benchmarkIdentity(group*5 + level + 10000)})
		}
		state.Burials = append(state.Burials, Burial{
			ID: int64(i + 1), Path: filepath.Join(parent, fmt.Sprintf("artefact-%04d.log", i)),
			Grave:    fmt.Sprintf(`C:\data\graveyard\%d-20261111T120000-abcdef\artefact-%04d.log`, i+1, i),
			Identity: benchmarkIdentity(i + 1), GraveID: benchmarkIdentity(i + 2000), Parents: parents,
			LongNames: true, Size: 4096, BuriedAt: now, PurgeAt: now.Add(7 * 24 * time.Hour),
			Reason: "bury", Source: "cli", Project: `C:\agent\workspace\example-project`, Session: "agent-session",
			Operation: &BurialOperation{ID: fmt.Sprintf("batch-%04d", i/80), StartedAt: now, Project: `C:\agent\workspace\example-project`, Session: "agent-session"},
		})
	}
	for i := range 130 {
		state.Items = append(state.Items, Item{ID: int64(i + 1301), Rev: 1,
			Path:     fmt.Sprintf(`C:\agent\workspace\example-project\scratch\live-%04d.log`, i),
			Identity: benchmarkIdentity(i + 4000), CreatedAt: now, ExpiresAt: now.Add(time.Hour), Source: "rule"})
	}
	for i := range 35 {
		state.Proposals = append(state.Proposals, Proposal{ID: int64(i + 1431), Kind: ProposalLeftovers,
			Project: `C:\agent\workspace\example-project`, Session: fmt.Sprintf("session-%d", i), CreatedAt: now,
			Candidates: []Candidate{{Path: fmt.Sprintf(`C:\agent\workspace\example-project\leftover-%04d`, i), Lifespan: "3d", Size: 4096}}})
	}
	return state
}

func benchmarkIdentity(n int) *Identity {
	return &Identity{Volume: `C:\`, Serial: 123456789012345, FileID: fmt.Sprintf("%032x", n)}
}

// Field sizes include the JSON key, colon and trailing comma. Repeated full
// values are counted separately from prefixes so unique evidence stays visible.
func TestLedgerFieldSizes(t *testing.T) {
	for _, version := range []int{15, 16} {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			st := benchmarkLedger()
			st.Version = version
			reportLedgerFieldSizes(t, st)
		})
	}
}

func reportLedgerFieldSizes(t *testing.T, st State) {
	t.Helper()
	data, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Burials []map[string]json.RawMessage `json:"burials"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	totals := map[string]int{}
	values := map[string]map[string]bool{}
	for _, row := range wire.Burials {
		for key, value := range row {
			totals[key] += len(key) + len(value) + 4
			if values[key] == nil {
				values[key] = map[string]bool{}
			}
			values[key][string(value)] = true
		}
	}
	keys := make([]string, 0, len(totals))
	for key := range totals {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		t.Logf("%s: %d B, %d distinct", key, totals[key], len(values[key]))
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"burials", "parentChains", "operations", "volumes", "origins"} {
		if value, ok := top[key]; ok {
			t.Logf("top-level %s: %d B", key, len(key)+len(value)+4)
		}
	}
	t.Logf("ledger v%d: %d B (without final newline)", st.Version, len(data))
}

func parentEvidence() []BurialParent {
	return []BurialParent{
		{Path: `\\?\C:\` + strings.Repeat(`long-directory\`, 30), Identity: Identity{Volume: `\\?\D:\`, Serial: ^uint64(0), FileID: "abcdef0123456789abcdef0123456789"}, Reparse: true},
		{Path: `C:\mixed\Case`, Identity: Identity{Volume: `d:\`, Serial: 9, FileID: "ABCDEF0123456789ABCDEF0123456789"}},
		{Path: `\\?\UNC\Server\Share\目录`, Identity: Identity{Volume: `\\?\UNC\Server\Share\`, Serial: 7, FileID: "00000000000000000000000000000001"}, Reparse: true},
		{Path: `C:\`, Identity: Identity{Volume: `C:\`, Serial: 0, FileID: "00000000000000000000000000000002"}},
	}
}

func TestLedgerParentChainsRoundTrip(t *testing.T) {
	chain := parentEvidence()
	st := State{Version: 15, Burials: []Burial{{ID: 1, Parents: chain}, {ID: 2, Parents: slices.Clone(chain)}, {ID: 3}, {ID: 4, Parents: []BurialParent{}}},
		RestoreCleanups: []RestoreCleanup{{Burial: Burial{ID: 5, Parents: chain}}}}
	for _, field := range []string{"path", "volume", "serial", "fileId", "reparse"} {
		other := slices.Clone(chain)
		switch field {
		case "path":
			other[0].Path += "x"
		case "volume":
			other[0].Identity.Volume = `D:\`
		case "serial":
			other[0].Identity.Serial--
		case "fileId":
			other[0].Identity.FileID = "different"
		case "reparse":
			other[0].Reparse = false
		}
		st.Burials = append(st.Burials, Burial{ID: int64(len(st.Burials) + 1), Parents: other})
	}
	data, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	wire := ledgerState{plainState: new(plainState)}
	if err := json.Unmarshal(data, &wire); err != nil || len(wire.ParentChains) != 7 || bytes.Contains(data, []byte(`"parents":`)) {
		t.Fatalf("deduplication lost distinct evidence: %s %v", data, err)
	}
	assertLedgerRoundTrip(t, st, data)
}

func assertLedgerRoundTrip(t *testing.T, want State, data []byte) {
	t.Helper()
	var got State
	if err := json.Unmarshal(data, &got); err != nil || !reflect.DeepEqual(want, got) {
		t.Fatalf("round trip changed evidence: want=%+v got=%+v err=%v", want, got, err)
	}
	got.Burials[0].Parents[0].Reparse = false
	if !got.Burials[1].Parents[0].Reparse || !got.RestoreCleanups[0].Burial.Parents[0].Reparse {
		t.Fatal("decoded chains share mutable parent evidence")
	}
}

func TestLedgerCompactAndIndentedReads(t *testing.T) {
	want := benchmarkLedger()
	want.Version = 16
	for _, indent := range []bool{false, true} {
		data, err := json.Marshal(want)
		if indent {
			data, err = json.MarshalIndent(want, "", "  ")
		}
		if err != nil {
			t.Fatal(err)
		}
		store := Store{Home: t.TempDir()}
		if err := os.WriteFile(store.statePath(), data, 0600); err != nil {
			t.Fatal(err)
		}
		got, err := store.Inspect()
		got.graveyardLoaded, got.graveyardOriginal = false, [32]byte{}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("full ledger fields changed, indented=%v: %v", indent, err)
		}
	}
}

func TestLedgerRejectsInvalidParentReferences(t *testing.T) {
	for _, data := range []string{
		`{"version":15,"burials":[{"id":1,"parentRef":0}]}`,
		`{"version":15,"burials":[{"id":1,"parentRef":-1}],"parentChains":[[]]}`,
		`{"version":15,"burials":[{"id":1,"parentRef":1}],"parentChains":[[]]}`,
		`{"version":15,"burials":[{"id":1,"parentRef":0}],"parentChains":[null]}`,
		`{"version":15,"burials":[{"id":1,"parents":[],"parentRef":0}],"parentChains":[[]]}`,
		`{"version":15,"restoreCleanups":[{"burial":{"id":1,"parentRef":0}}]}`,
		`{"version":16,"burials":[{"parentRef":0}]}`,
	} {
		var st State
		if err := json.Unmarshal([]byte(data), &st); err == nil {
			t.Fatalf("invalid parent evidence accepted: %s", data)
		}
	}
}

func operationEvidence() []*BurialOperation {
	base := BurialOperation{ID: "batch-1", StartedAt: time.Date(2026, 11, 11, 12, 0, 0, 123, time.UTC),
		Project: `C:\project\Case`, Session: "claude:session"}
	result := []*BurialOperation{nil, {}, &base}
	for _, field := range []string{"id", "startedAt", "offset", "session", "project"} {
		other := base
		switch field {
		case "id":
			other.ID = "batch-2"
		case "startedAt":
			other.StartedAt = other.StartedAt.Add(time.Nanosecond)
		case "offset":
			other.StartedAt = other.StartedAt.In(time.FixedZone("", 3600))
		case "session":
			other.Session = ""
		case "project":
			other.Project = `C:\project\case`
		}
		result = append(result, &other)
	}
	return result
}

func TestLedgerOperationsRoundTrip(t *testing.T) {
	st := State{Version: 16}
	for i, op := range operationEvidence() {
		st.Burials = append(st.Burials, Burial{ID: int64(i + 1), Operation: op})
		if op != nil {
			copy := *op
			op = &copy
		}
		st.RestoreCleanups = append(st.RestoreCleanups, RestoreCleanup{Burial: Burial{ID: int64(i + 20), Operation: op}})
	}
	data, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Operations []json.RawMessage `json:"operations"`
	}
	if err := json.Unmarshal(data, &wire); err != nil || len(wire.Operations) != 7 || bytes.Contains(data, []byte(`"operation":`)) {
		t.Fatalf("operation deduplication lost evidence: %s %v", data, err)
	}
	var got State
	if err := json.Unmarshal(data, &got); err != nil || !reflect.DeepEqual(st, got) {
		t.Fatalf("operations changed: %+v %v", got, err)
	}
	got.Burials[2].Operation.Session = "changed"
	if got.RestoreCleanups[2].Burial.Operation.Session != st.Burials[2].Operation.Session {
		t.Fatal("decoded operations share mutable evidence")
	}
}

func TestLedgerLegacyInlineOperations(t *testing.T) {
	for _, version := range []int{14, 15} {
		data := []byte(fmt.Sprintf(`{"version":%d,"burials":[{"id":1},{"id":2,"operation":null},{"id":3,"operation":{}},{"id":4,"operation":{"id":"batch","startedAt":"2026-11-11T12:00:00Z","session":"s","project":"p"}}],"restoreCleanups":[{"burial":{"id":5,"operation":{"id":"batch","startedAt":"2026-11-11T12:00:00Z","session":"s","project":"p"}}}]}`, version))
		var st State
		if err := json.Unmarshal(data, &st); err != nil {
			t.Fatal(err)
		}
		want := &BurialOperation{ID: "batch", StartedAt: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC), Session: "s", Project: "p"}
		if st.Burials[0].Operation != nil || st.Burials[1].Operation != nil || !reflect.DeepEqual(st.Burials[2].Operation, &BurialOperation{}) || !reflect.DeepEqual(st.Burials[3].Operation, want) || !reflect.DeepEqual(st.RestoreCleanups[0].Burial.Operation, want) {
			t.Fatalf("legacy operations changed: %+v", st)
		}
	}
}

func TestLedgerRejectsInvalidOperationReferences(t *testing.T) {
	for _, data := range []string{
		`{"version":16,"burials":[{"operationRef":0}]}`,
		`{"version":16,"burials":[{"operationRef":-1}],"operations":[{}]}`,
		`{"version":16,"burials":[{"operationRef":1}],"operations":[{}]}`,
		`{"version":16,"burials":[{"operationRef":0}],"operations":[null]}`,
		`{"version":16,"burials":[{"operation":{},"operationRef":0}],"operations":[{}]}`,
		`{"version":16,"burials":[{"operation":null}]}`,
		`{"version":16,"restoreCleanups":[{"burial":{"operationRef":0}}]}`,
		`{"version":15,"burials":[{"operationRef":0}],"operations":[{}]}`,
		`{"version":14,"burials":[{"operationRef":0}],"operations":[{}]}`,
	} {
		var st State
		if err := json.Unmarshal([]byte(data), &st); err == nil {
			t.Fatalf("invalid operation evidence accepted: %s", data)
		}
	}
}

func TestLedgerNewerVersionIsNotCorrupt(t *testing.T) {
	for _, suffix := range []string{`"burials":[{"operationRef":0}]`, `"operations":"future layout"`} {
		store := Store{Home: t.TempDir()}
		data := []byte(fmt.Sprintf(`{"version":%d,%s}`, stateVersion+1, suffix))
		if err := os.WriteFile(store.statePath(), data, 0600); err != nil {
			t.Fatal(err)
		}
		_, err := store.Inspect()
		var newer newerStoreError
		if !errors.As(err, &newer) || strings.Contains(err.Error(), "corrupt") {
			t.Fatalf("newer ledger reported as corruption: %v", err)
		}
		if err := store.Update(func(*State) error { t.Fatal("newer ledger reached mutation"); return nil }); !errors.As(err, &newer) {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(store.statePath())
		if !bytes.Equal(got, data) {
			t.Fatal("newer ledger was changed")
		}
	}
}

func TestLedgerCollectsUnusedOperations(t *testing.T) {
	st := State{Version: 16, Burials: []Burial{{ID: 1, Operation: operationEvidence()[2]}}}
	st.RestoreCleanups = []RestoreCleanup{{Burial: Burial{ID: 2, Operation: operationEvidence()[2]}}}
	for _, want := range []int{1, 1, 0} {
		data, err := json.Marshal(st)
		var wire struct {
			Operations []json.RawMessage `json:"operations"`
		}
		if err != nil || json.Unmarshal(data, &wire) != nil || len(wire.Operations) != want {
			t.Fatalf("unused operations retained: want %d, data=%s err=%v", want, data, err)
		}
		var decoded State
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		st = decoded
		if len(st.Burials) > 0 {
			st.Burials = nil
		} else {
			st.RestoreCleanups = nil
		}
	}
}

func TestLedgerMetadataRoundTrip(t *testing.T) {
	st := State{Version: 16}
	ids := []*Identity{nil, {}, benchmarkIdentity(1), benchmarkIdentity(2),
		{Volume: `c:\`, Serial: 123456789012345, FileID: "other"},
		{Volume: `C:\`, Serial: ^uint64(0), FileID: "other"},
		{Volume: `\\?\UNC\Server\Share\`, Serial: 0, FileID: "other"}}
	for i, id := range ids {
		b := Burial{ID: int64(i + 1), Identity: id, GraveID: id, Project: `C:\project`, Session: "session"}
		if i == 0 {
			b.Project, b.Session = "", ""
		}
		if i == 1 {
			b.Project = `c:\project`
		}
		if i == 2 {
			b.Session = ""
		}
		st.Burials = append(st.Burials, b)
		st.RestoreCleanups = append(st.RestoreCleanups, RestoreCleanup{Burial: b})
	}
	data, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	wire := ledgerState{plainState: new(plainState)}
	if err := json.Unmarshal(data, &wire); err != nil || len(wire.Volumes) != 5 || len(wire.Origins) != 3 {
		t.Fatalf("metadata lost distinct evidence: %s %v", data, err)
	}
	var got State
	if err := json.Unmarshal(data, &got); err != nil || !reflect.DeepEqual(st, got) {
		t.Fatalf("metadata changed: %+v %v", got, err)
	}
	got.Burials[2].Identity.Serial = 0
	if got.Burials[2].GraveID.Serial == 0 || got.Burials[3].Identity.Serial == 0 || got.RestoreCleanups[2].Burial.Identity.Serial == 0 {
		t.Fatal("decoded identities share mutable evidence")
	}
	data, err = json.Marshal(st.Burials[2])
	if err != nil || !bytes.Contains(data, []byte(`"volume":`)) || !bytes.Contains(data, []byte(`"project":`)) || bytes.Contains(data, []byte(`Ref":`)) {
		t.Fatalf("standalone burial lost inline evidence: %s %v", data, err)
	}
}

func TestLedgerRejectsInvalidMetadataReferences(t *testing.T) {
	for _, row := range []string{
		`"identity":{"fileId":"a"}`,
		`"identity":{"volumeRef":-1}`,
		`"identity":{"volumeRef":1}`,
		`"identity":{"volumeRef":0,"serial":0}`,
		`"graveId":{"volumeRef":0,"volume":"C:"}`,
		`"originRef":-1`, `"originRef":1`, `"originRef":0,"project":""`, `"session":null`,
	} {
		for _, array := range []string{`"burials":[{%s}]`, `"restoreCleanups":[{"burial":{%s}}]`} {
			data := fmt.Sprintf(`{"version":16,"volumes":[{}],"origins":[{}],%s}`, fmt.Sprintf(array, row))
			var st State
			if err := json.Unmarshal([]byte(data), &st); err == nil {
				t.Fatalf("invalid metadata accepted: %s", data)
			}
		}
	}
	for _, data := range []string{
		`{"version":16,"burials":[{"identity":{"volumeRef":0}}],"volumes":[null]}`,
		`{"version":16,"burials":[{"originRef":0}],"origins":[null]}`,
		`{"version":16,"burials":[{"graveId":{"volumeRef":0}}]}`,
		`{"version":16,"burials":[{"originRef":0}]}`,
		`{"version":15,"burials":[{"identity":{"volumeRef":0}}],"volumes":[{}]}`,
		`{"version":14,"burials":[{"originRef":0}],"origins":[{}]}`,
	} {
		var st State
		if err := json.Unmarshal([]byte(data), &st); err == nil {
			t.Fatalf("invalid metadata accepted: %s", data)
		}
	}
}

func TestLedgerCollectsUnusedMetadata(t *testing.T) {
	st := State{Version: 16, Burials: []Burial{{Identity: benchmarkIdentity(1), Project: "project"}}}
	st.RestoreCleanups = []RestoreCleanup{{Burial: st.Burials[0]}}
	for _, want := range []int{1, 1, 0} {
		data, err := json.Marshal(st)
		wire := ledgerState{plainState: new(plainState)}
		if err != nil || json.Unmarshal(data, &wire) != nil || len(wire.Volumes) != want || len(wire.Origins) != want {
			t.Fatalf("unused metadata retained: want %d, data=%s err=%v", want, data, err)
		}
		var decoded State
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		st = decoded
		if len(st.Burials) > 0 {
			st.Burials = nil
		} else {
			st.RestoreCleanups = nil
		}
	}
}

func TestLedgerCollectsUnusedParentChains(t *testing.T) {
	st := State{Version: 15, Burials: []Burial{{ID: 1, Parents: parentEvidence()}, {ID: 2, Parents: parentEvidence()}}}
	st.RestoreCleanups = []RestoreCleanup{{Burial: Burial{ID: 3, Parents: []BurialParent{{Path: `D:\`}}}}}
	for _, want := range []int{2, 2, 1, 0} {
		data, err := json.Marshal(st)
		wire := ledgerState{plainState: new(plainState)}
		if err != nil || json.Unmarshal(data, &wire) != nil || len(wire.ParentChains) != want {
			t.Fatalf("unused chains retained: want %d, data=%s err=%v", want, data, err)
		}
		if len(st.Burials) > 0 {
			st.Burials = st.Burials[1:]
		} else {
			st.RestoreCleanups = nil
		}
	}
}
