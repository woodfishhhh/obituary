//go:build windows

package proposals

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"golang.org/x/sys/windows"
)

func TestProposalFailuresReturnNonzeroAfterReporting(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, approve := range []bool{false, true} {
		for _, candidateError := range []bool{false, true} {
			if candidateError && !approve {
				continue
			}
			for _, asJSON := range []bool{false, true} {
				t.Run(strings.Join([]string{map[bool]string{true: "approve", false: "reject"}[approve], map[bool]string{true: "candidate", false: "proposal"}[candidateError], map[bool]string{true: "JSON", false: "text"}[asJSON]}, "/"), func(t *testing.T) {
					starts := clitest.IsolateRunner(t)
					repo, store := clitest.CommandRepo(t)
					path := filepath.Join(repo, "debug.log")
					clitest.WriteFixture(t, path, "debug")
					candidates := []core.Candidate{{Path: path, Lifespan: "1d"}}
					if candidateError {
						candidates = append(candidates, core.Candidate{Path: filepath.Join(repo, "gone"), Lifespan: "1d"})
					}
					clitest.AddProposal(t, store, repo, candidates...)
					args := []string{"1"}
					if !candidateError {
						args = append(args, "999")
					}
					if asJSON {
						args = append(args, "--json")
					}
					out, stderr, err := clitest.Execute(t, NewDecideCmd("decide", "", approve), args...)
					if !errors.Is(err, cmdutil.ErrReported) {
						t.Fatalf("error = %v, output=%s", err, out)
					}
					if asJSON && (!json.Valid([]byte(out)) || !strings.Contains(out, `"error"`) || stderr != "") {
						t.Fatalf("JSON failure not reported cleanly: %s %s", out, stderr)
					}
					wantStarts := 0
					if approve {
						wantStarts = 1
					}
					if len(*starts) != wantStarts {
						t.Fatalf("partial success runner starts = %v", *starts)
					}
				})
			}
		}
	}
}

func TestApproveRetainsTransientCandidatesAndReportsPartialSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, fault := range []string{"git", "add", "store"} {
		for _, asJSON := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%v", fault, asJSON), func(t *testing.T) {
				starts := clitest.IsolateRunner(t)
				root, store := clitest.CommandRepo(t)
				first, second := filepath.Join(root, "first.log"), filepath.Join(root, "second.log")
				clitest.WriteFixture(t, first, "one")
				clitest.WriteFixture(t, second, "two")
				clitest.AddProposal(t, store, root, core.Candidate{Path: first, Lifespan: "1d"}, core.Candidate{Path: second, Lifespan: "1d"})
				svc, err := app.Open()
				if err != nil {
					t.Fatal(err)
				}
				oldOpen := cmdutil.Open
				cmdutil.Open = func() (*core.Service, error) { return svc, nil }
				t.Cleanup(func() { cmdutil.Open = oldOpen })
				originalPath := os.Getenv("PATH")
				blocked := windows.InvalidHandle
				unblock := func() {
					if blocked != windows.InvalidHandle {
						windows.CloseHandle(blocked)
						blocked = windows.InvalidHandle
					}
				}
				t.Cleanup(unblock)
				switch fault {
				case "git":
					emptyPath := t.TempDir()
					svc.FS = approvalFaultFS{FS: svc.FS, beforeRealPath: func(path string) {
						if strings.EqualFold(path, second) {
							t.Setenv("PATH", emptyPath)
						}
					}}
				case "store":
					calls := 0
					svc.FS = approvalFaultFS{FS: svc.FS, beforeIdentify: func(path string) {
						if !strings.EqualFold(path, second) {
							return
						}
						calls++
						if calls == 2 { // after vetting, before Add writes the ledger
							p, err := windows.UTF16PtrFromString(filepath.Join(store.Home, "store.json"))
							if err != nil {
								t.Fatal(err)
							}
							// Allow reads but deny replacement: both Add and proposal
							// finalization fail their actual store writes.
							blocked, err = windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
							if err != nil {
								t.Fatal(err)
							}
						}
					}}
				default:
					// A forward clock adjustment between preparing AddOptions and
					// Add rejects the second Add before its store write.
					now, calls := time.Now(), 0
					svc.Now = func() time.Time {
						calls++
						if calls == 4 {
							return now.Add(48 * time.Hour)
						}
						return now
					}
				}
				args := []string{"1"}
				if asJSON {
					args = append(args, "--json")
				}
				out, stderr, err := clitest.Execute(t, NewDecideCmd("approve", "", true), args...)
				unblock()
				if err == nil {
					t.Error("partial transient failure must exit nonzero")
				}
				if asJSON {
					var got []core.ProposalResult
					if json.Unmarshal([]byte(out), &got) != nil || len(got) != 1 || len(got[0].Results) != 2 || got[0].Results[0].Item == nil || got[0].Results[0].Item.Path != first {
						t.Errorf("completed result missing from JSON: %s (%s)", out, stderr)
					}
				} else if !strings.Contains(out, first) || !strings.Contains(out, "expires") {
					t.Errorf("completed result missing from text: %s (%s)", out, stderr)
				}
				if len(*starts) != 1 {
					t.Errorf("partial success must ensure runner: %v", *starts)
				}
				pending, _, err := clitest.Execute(t, NewCmd(), "--json")
				var proposals []core.Proposal
				wantPending := 1
				if fault == "store" {
					wantPending = 2 // the failed final write must leave the original intact
				}
				if err != nil || json.Unmarshal([]byte(pending), &proposals) != nil || len(proposals) != 1 || len(proposals[0].Candidates) != wantPending || proposals[0].Candidates[wantPending-1].Path != second {
					t.Fatalf("failed candidate must remain pending: %s, %v", pending, err)
				}
				t.Setenv("PATH", originalPath)
				if fs, ok := svc.FS.(approvalFaultFS); ok {
					svc.FS = fs.FS
				}
				svc.Now = nil
				out, stderr, err = clitest.Execute(t, NewDecideCmd("approve", "", true), fmt.Sprint(proposals[0].ID), "--json")
				st, readErr := store.Read()
				if (err != nil && (fault != "store" || !errors.Is(err, cmdutil.ErrReported))) || readErr != nil || len(st.Items) != 2 || len(st.Proposals) != 0 {
					t.Fatalf("retry approval: %s %s, %v; ledger %+v, %v", out, stderr, err, st, readErr)
				}
			})
		}
	}
}
