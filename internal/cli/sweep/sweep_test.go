//go:build windows

package sweep

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"golang.org/x/sys/windows"
)

func TestReapReportsRecoveryInTextAndJSON(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		for _, asJSON := range []bool{false, true} {
			t.Run(fmt.Sprintf("corrupt=%v/json=%v", corrupt, asJSON), func(t *testing.T) {
				_, _, p := clitest.PendingCommandBurial(t, false)
				want := core.ActionBuried
				if corrupt {
					clitest.WriteFixture(t, p.Manifest, "{broken")
					want = core.ActionBlocked
				}
				var args []string
				if asJSON {
					args = append(args, "--json")
				}
				out, _, err := clitest.Execute(t, NewCmd(), args...)
				if (err != nil) != corrupt || !strings.Contains(out, want) || strings.Contains(out, "nothing is due") {
					t.Fatalf("wrong recovery receipt/status: %s, %v", out, err)
				}
				if asJSON {
					var rows []core.Outcome
					if json.Unmarshal([]byte(out), &rows) != nil || len(rows) != 1 || rows[0].Action != want || rows[0].Item.ID != p.Burial.ID || rows[0].Detail == "" {
						t.Fatalf("JSON missing recovery details: %s", out)
					}
					if !corrupt && (rows[0].Burial == nil || rows[0].Burial.Grave != p.Burial.Grave || !rows[0].Burial.PurgeAt.Equal(p.Burial.PurgeAt) || rows[0].Burial.Size != 4) {
						t.Fatalf("JSON missing recovered burial record: %s", out)
					}
				}
			})
		}
	}
}

func TestReapReportsRecoveryPending(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, format := range []string{"text", "json"} {
		t.Run(format, func(t *testing.T) {
			repo, store := clitest.CommandRepo(t)
			svc, err := cmdutil.Open()
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(repo, "payload.txt")
			clitest.WriteFixture(t, path, "keep")
			now := time.Now()
			if rows, err := svc.Add([]string{path}, core.AddOptions{ExpiresAt: now.Add(time.Hour)}); err != nil || core.Failed(rows) {
				t.Fatalf("add: %+v %v", rows, err)
			}
			svc.Now = func() time.Time { return now.Add(2 * time.Hour) }
			var handle windows.Handle
			svc.FS = lockedBurialFS{FS: svc.FS, lock: func() {
				name, err := windows.UTF16PtrFromString(filepath.Join(store.Home, "store.json"))
				if err != nil {
					t.Fatal(err)
				}
				handle, err = windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
				if err != nil {
					t.Fatal(err)
				}
			}}
			defer func() {
				if handle != 0 {
					windows.CloseHandle(handle)
				}
			}()
			previous := cmdutil.Open
			cmdutil.Open = func() (*core.Service, error) { return svc, nil }
			t.Cleanup(func() { cmdutil.Open = previous })
			args := []string{}
			if format == "json" {
				args = append(args, "--json")
			}
			out, _, err := clitest.Execute(t, NewCmd(), args...)
			if err == nil || !strings.Contains(out, core.ActionRecoveryPending) || !strings.Contains(out, "obit reap") {
				t.Fatalf("reap omitted recovery receipt: %s %v", out, err)
			}
			if format == "json" {
				var rows []core.Outcome
				if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) != 1 || rows[0].Action != core.ActionRecoveryPending || rows[0].Grave == "" {
					t.Fatalf("pending JSON: %s %v", out, err)
				}
				if data, err := os.ReadFile(rows[0].Grave); err != nil || string(data) != "keep" {
					t.Fatalf("pending JSON lost location: %s %v", data, err)
				}
			} else if !strings.Contains(out, filepath.Join(store.Home, "graveyard")) {
				t.Fatalf("pending text lost grave location: %s", out)
			}
		})
	}
}

func TestReapRefreshesProposalsExceptDryRun(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	root, store := clitest.CommandRepo(t)
	clitest.AddProposal(t, store, root, core.Candidate{Path: filepath.Join(root, "gone.txt"), Lifespan: "1d"})
	if _, _, err := clitest.Execute(t, NewCmd(), "--dry-run"); err != nil {
		t.Fatal(err)
	}
	st, err := store.Read()
	if err != nil || len(st.Proposals) != 1 {
		t.Fatalf("dry run changed proposals: %+v %v", st.Proposals, err)
	}
	if _, _, err := clitest.Execute(t, NewCmd()); err != nil {
		t.Fatal(err)
	}
	st, err = store.Read()
	if err != nil || len(st.Proposals) != 0 || len(st.LeftoverRejections) != 0 {
		t.Fatalf("reap retained stale proposals or rejected them: %+v %v", st.Proposals, err)
	}
}
