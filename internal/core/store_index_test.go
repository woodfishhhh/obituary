package core

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
		if !st.graveyardMayMatch(path, nil) {
			t.Fatal("complete safety index lost equivalent path", path)
		}
	}
	if runtime.GOOS == "windows" {
		id.Volume = `\\?\c:\`
	}
	if !st.graveyardMayMatch("unrelated", &id) {
		t.Fatal("complete safety index lost identity spelling")
	}
	if _, ok := slices.BinarySearch(st.GraveyardIndex.Keys, graveyardKey("id", "7")); !ok || st.GraveyardIndex.MaxID != 9 {
		t.Fatal("complete safety index lost child or cleanup allocation")
	}
}
