package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// Measure the whole file-backed transaction, including production fsyncs.
func BenchmarkStoreLedger(b *testing.B) {
	previous := syncFile
	syncFile = (*os.File).Sync
	b.Cleanup(func() { syncFile = previous })
	for _, operation := range []string{"load", "save", "update-single"} {
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
			b.ReportAllocs()
			b.SetBytes(info.Size())
			for b.Loop() {
				switch operation {
				case "load":
					_, _, err = store.load()
				case "save":
					err = store.save(state)
				case "update-single":
					err = store.Update(func(st *State) error { st.Items[0].Rev++; return nil })
				}
				if err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(info.Size()), "ledger-B")
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
			Operation: &BurialOperation{ID: fmt.Sprintf("batch-%04d", i/10), StartedAt: now, Project: `C:\agent\workspace\example-project`, Session: "agent-session"},
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
