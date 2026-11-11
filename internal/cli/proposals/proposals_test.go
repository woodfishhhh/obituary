package proposals

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/core"
)

func TestProposalItemCounts(t *testing.T) {
	for _, n := range []int{1, 2} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			root, store := clitest.CommandDir(t)
			var candidates []core.Candidate
			for i := range n {
				path := filepath.Join(root, fmt.Sprintf("file%d.txt", i))
				clitest.WriteFixture(t, path, "keep")
				candidates = append(candidates, core.Candidate{Path: path, Lifespan: "1d"})
			}
			clitest.AddProposal(t, store, root, candidates...)
			out, _, err := clitest.Execute(t, NewCmd())
			want := "1 item,"
			if n == 2 {
				want = "2 items,"
			}
			if err != nil || !strings.Contains(out, want) {
				t.Fatalf("proposal = %s, %v", out, err)
			}
		})
	}
}

func TestProposalsOmitsMissingCandidates(t *testing.T) {
	root, store := clitest.CommandDir(t)
	keep, gone := filepath.Join(root, "keep.txt"), filepath.Join(root, "gone.txt")
	clitest.WriteFixture(t, keep, "keep")
	clitest.AddProposal(t, store, root, core.Candidate{Path: keep, Lifespan: "1d"}, core.Candidate{Path: gone, Lifespan: "1d"})
	t.Setenv("PATH", t.TempDir())
	for _, args := range [][]string{nil, {"--json"}} {
		out, _, err := clitest.Execute(t, NewCmd(), args...)
		if err != nil || !strings.Contains(out, "keep.txt") || strings.Contains(out, "gone.txt") {
			t.Fatalf("CLI displayed missing candidates: %s %v", out, err)
		}
	}
	if err := os.Remove(keep); err != nil {
		t.Fatal(err)
	}
	out, _, err := clitest.Execute(t, NewCmd())
	if err != nil || !strings.Contains(out, "no pending proposals") {
		t.Fatalf("CLI displayed empty proposal: %s %v", out, err)
	}
}
