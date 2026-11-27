package ledger

import (
	"runtime"
	"slices"
	"testing"
)

func TestGraveyardSafetyIndexPreservesConservativeMatches(t *testing.T) {
	id := *benchmarkIdentity(1)
	st := State{Burials: []Burial{{ID: 1, Path: `C:\K\Σ\payload`, Identity: &id, Contains: []int64{7}}},
		RestoreCleanups: []RestoreCleanup{{Burial: Burial{ID: 9, Path: `C:\cleanup`}}}}
	st.GraveyardIndex = buildGraveyardIndex(st)
	for _, path := range []string{`c:\k\σ\payload`, `C:\K\ς\PAYLOAD`, `C:\cleanup`} {
		if !GraveyardMayMatch(&st, path, nil) {
			t.Fatal("complete safety index lost equivalent path", path)
		}
	}
	if runtime.GOOS == "windows" {
		id.Volume = `\\?\c:\`
	}
	if !GraveyardMayMatch(&st, "unrelated", &id) {
		t.Fatal("complete safety index lost identity spelling")
	}
	if _, ok := slices.BinarySearch(st.GraveyardIndex.Keys, graveyardKey("id", "7")); !ok || st.GraveyardIndex.MaxID != 9 {
		t.Fatal("complete safety index lost child or cleanup allocation")
	}
}

func TestGraveyardSafetyIndexExcludesExternalVolumes(t *testing.T) {
	st := State{Burials: []Burial{
		{ID: 1, Kind: KindContainer, Path: "docker:web (aaaaaaaaaaaa)"},
		{ID: 2, Kind: KindDockerVolume, Path: "cache"},
	}}
	idx := buildGraveyardIndex(st)
	if len(idx.Volumes) != 0 {
		t.Fatalf("external burials leaked into filesystem volumes: %v", idx.Volumes)
	}
	if idx.MaxID != 2 {
		t.Fatalf("MaxID = %d, want 2", idx.MaxID)
	}
}
