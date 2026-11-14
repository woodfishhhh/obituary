package proposals

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestProposalsListsWhatWentInWithoutAsking(t *testing.T) {
	root, store := clitest.CommandDir(t)
	path := filepath.Join(root, "debug.log")
	clitest.WriteFixture(t, path, "x")
	if err := store.Update(func(st *core.State) error {
		st.Items = append(st.Items, core.Item{ID: 7, Rev: 1, Path: path, Source: "rule", Project: root, ExpiresAt: time.Now().Add(48 * time.Hour)})
		st.AddAutoReceipts(core.AutoReceipt{ID: 7, Class: core.AutoCreated, Project: root, At: time.Now()})
		st.NextID = 8
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	out, _, err := clitest.Execute(t, NewCmd())
	for _, want := range []string{"no pending proposals", "registered without asking", "#7", "created", "debug.log", "obit spare <id>", "proposal mode tiered", `[proposals] mode = "auto"`} {
		if err != nil || !strings.Contains(out, want) {
			t.Fatalf("output lacks %q: %s %v", want, out, err)
		}
	}
	clitest.WriteFixture(t, filepath.Join(os.Getenv("OBIT_CONFIG_DIR"), "config.toml"), "[proposals]\nmode = \"auto\"\n")
	if out, _, err := clitest.Execute(t, NewCmd()); err != nil || !strings.Contains(out, "proposal mode auto") || !strings.Contains(out, `mode = "tiered"`) {
		t.Fatalf("auto mode hint: %s %v", out, err)
	}
	if out, _, err := clitest.Execute(t, NewCmd(), "--json"); err != nil || strings.TrimSpace(out) != "[]" {
		t.Fatalf("--json keeps listing only proposals: %s %v", out, err)
	}
}
