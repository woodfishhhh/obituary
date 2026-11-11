package testenv

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

// InMutationShard lets CI split the compiled mutation suites across runners.
// OBIT_MUTATION_SHARD="k/n" keeps the cases whose index leaves remainder k
// when divided by n; unset keeps every case, so a local run checks them all.
func InMutationShard(t testing.TB, index int) bool {
	t.Helper()
	k, n, err := MutationShard(os.Getenv("OBIT_MUTATION_SHARD"))
	if err != nil {
		t.Fatal(err)
	}
	return index%n == k
}

// MutationShard parses "k/n"; an empty value is the single shard "0/1".
func MutationShard(spec string) (k, n int, err error) {
	if spec == "" {
		return 0, 1, nil
	}
	left, right, ok := strings.Cut(spec, "/")
	k, errK := strconv.Atoi(left)
	n, errN := strconv.Atoi(right)
	if !ok || errK != nil || errN != nil || n < 1 || k < 0 || k >= n {
		return 0, 0, fmt.Errorf("OBIT_MUTATION_SHARD must look like 0/4 with 0 <= k < n: %q", spec)
	}
	return k, n, nil
}
