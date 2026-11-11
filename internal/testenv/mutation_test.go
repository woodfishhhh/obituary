package testenv

import "testing"

func TestMutationShardParsing(t *testing.T) {
	for spec, want := range map[string][2]int{"": {0, 1}, "0/1": {0, 1}, "2/6": {2, 6}, "5/6": {5, 6}} {
		k, n, err := MutationShard(spec)
		if err != nil || k != want[0] || n != want[1] {
			t.Errorf("%q: got %d/%d %v, want %d/%d", spec, k, n, err, want[0], want[1])
		}
	}
	for _, spec := range []string{"6/6", "-1/6", "1/0", "1", "a/b", "1/2/3", " 1/2"} {
		if _, _, err := MutationShard(spec); err == nil {
			t.Errorf("%q accepted", spec)
		}
	}
}

func TestMutationShardsCoverEveryCaseOnce(t *testing.T) {
	const cases, shards = 491, 6
	seen := make([]int, cases)
	for k := range shards {
		t.Setenv("OBIT_MUTATION_SHARD", string(rune('0'+k))+"/6")
		for i := range cases {
			if InMutationShard(t, i) {
				seen[i]++
			}
		}
	}
	for i, n := range seen {
		if n != 1 {
			t.Fatalf("case %d ran in %d shards", i, n)
		}
	}
}
