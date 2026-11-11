//go:build windows

package purge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"golang.org/x/sys/windows"
)

func TestPurgeRecoveryReportsActualRestoredPath(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, format := range []string{"text", "dry-run", "json"} {
		t.Run(format, func(t *testing.T) {
			svc, paths, unlock := clitest.RestoreReceiptFixture(t)
			rows, err := svc.Resurrect(paths[:2])
			if err == nil || len(rows) != 2 || rows[1].Path == paths[1] {
				t.Fatalf("fixture did not interrupt suffix resurrection: %+v %v", rows, err)
			}
			actual := rows[1].Path
			unlock()
			previous := cmdutil.Open
			cmdutil.Open = func() (*core.Service, error) { return svc, nil }
			t.Cleanup(func() { cmdutil.Open = previous })
			args := []string{paths[1]}
			if format == "json" {
				args = append(args, "--json")
			}
			if format == "dry-run" {
				args = append(args, "--dry-run")
			}
			before, err := svc.Store.ReadSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			out, _, err := clitest.Execute(t, NewCmd(), args...)
			if err != nil {
				t.Fatalf("purge confirmation: %s %v", out, err)
			}
			if format == "json" {
				if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) != 1 || rows[0].Path != actual || rows[0].Action != core.ActionResurrected {
					t.Fatalf("purge JSON lost actual restored path: %s %v", out, err)
				}
			} else if !strings.Contains(out, actual) || format == "dry-run" && !strings.Contains(out, "would confirm an already completed resurrection") {
				t.Fatalf("purge recovery reported original path or future resurrection: %s", out)
			}
			after, err := svc.Store.ReadSnapshot()
			if err != nil || format == "dry-run" && !reflect.DeepEqual(before, after) {
				t.Fatalf("purge preview changed ledger: %v", err)
			}
			if data, err := os.ReadFile(paths[1]); err != nil || string(data) != "new work" {
				t.Fatal("purge confirmation changed conflicting original", err)
			}
			if data, err := os.ReadFile(actual); err != nil || string(data) != "buried" {
				t.Fatal("purge confirmation changed restored object", err)
			}
		})
	}
}

func TestPurgeHardFailureAfterAuditWarningReturnsFailure(t *testing.T) {
	clitest.IsolateRunner(t)
	repo, store := clitest.CommandDir(t)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	var refs []string
	var burials []core.Burial
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		path := filepath.Join(repo, name)
		clitest.WriteFixture(t, path, name)
		rows, err := svc.Bury([]string{path}, false, "")
		if err != nil || core.GraveFailed(rows) {
			t.Fatal(rows, err)
		}
		refs = append(refs, path)
		burials = append(burials, *rows[0].Burial)
	}
	audit := filepath.Join(store.Home, "audit.jsonl")
	if err := os.Remove(audit); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(audit, 0700); err != nil {
		t.Fatal(err)
	}
	injected := false
	svc.FS = approvalFaultFS{FS: svc.FS, beforeIdentify: func(path string) {
		if path == burials[1].Grave && !injected {
			injected = true
			if err := os.Mkdir(filepath.Join(store.Home, "store.json.tmp"), 0700); err != nil {
				t.Fatal(err)
			}
		}
	}}
	original := cmdutil.Open
	cmdutil.Open = func() (*core.Service, error) { return svc, nil }
	t.Cleanup(func() { cmdutil.Open = original })
	out, _, err := clitest.Execute(t, NewCmd(), append(refs, "--json")...)
	if err == nil || !strings.Contains(err.Error(), "write store") {
		t.Fatalf("audit warning swallowed purge hard failure: %s %v", out, err)
	}
	var rows []core.GraveResult
	if json.Unmarshal([]byte(out), &rows) != nil || len(rows) != 3 || rows[0].Action != core.ActionPurged || rows[1].Error == "" || rows[2].Action != core.ActionNotExecuted {
		t.Fatalf("purge hard failure lost per-item receipts: %s %v", out, err)
	}
	if _, err := os.Stat(burials[1].Grave); err != nil {
		t.Fatal("failed purge intent changed second grave", err)
	}
	if _, err := os.Stat(burials[2].Grave); err != nil {
		t.Fatal("unexecuted grave was changed", err)
	}
}

func TestPurgeRestoreCommitFailureReportsJSONReceipt(t *testing.T) {
	svc, paths, unlock := clitest.RestoreReceiptFixture(t)
	if _, err := svc.Resurrect(paths); err == nil {
		t.Fatal("fixture did not interrupt restore")
	}
	unlock()
	p, _ := windows.UTF16PtrFromString(filepath.Join(svc.Store.Home, "store.json"))
	h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	original := cmdutil.Open
	cmdutil.Open = func() (*core.Service, error) { return svc, nil }
	t.Cleanup(func() { cmdutil.Open = original })
	out, _, err := clitest.Execute(t, NewCmd(), paths[1], paths[2], "--json")
	want := filepath.Join(filepath.Dir(paths[1]), "b (resurrected).txt")
	var rows []core.GraveResult
	if err == nil || json.Unmarshal([]byte(out), &rows) != nil || len(rows) != 2 || rows[0].Path != want || rows[0].Error == "" || rows[1].Action != core.ActionNotExecuted {
		t.Fatalf("purge restore commit failure lost JSON receipt: %s %v", out, err)
	}
}

func TestPurgeCommandExplicitlyAbandonsRestore(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, refKind := range []string{"id", "path"} {
		t.Run(refKind, func(t *testing.T) {
			svc, paths, unlock := clitest.RestoreReceiptFixture(t)
			got, err := svc.Resurrect(paths[:2])
			if err == nil || len(got) != 2 {
				t.Fatalf("restore interruption fixture: %+v %v", got, err)
			}
			unlock()
			moved := filepath.Join(filepath.Dir(paths[1]), "elsewhere")
			if err := os.Mkdir(moved, 0700); err != nil {
				t.Fatal(err)
			}
			dest := filepath.Join(moved, "payload")
			if err := os.Rename(got[1].Path, dest); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(dest)
			original := cmdutil.Open
			cmdutil.Open = func() (*core.Service, error) { return svc, nil }
			t.Cleanup(func() { cmdutil.Open = original })
			ref := fmt.Sprint(got[1].Burial.ID)
			if refKind == "path" {
				ref = paths[1]
			}
			out, stderr, err := clitest.Execute(t, NewCmd(), ref, "--json")
			var rows []core.GraveResult
			if err != nil || json.Unmarshal([]byte(out), &rows) != nil || len(rows) != 1 || rows[0].Action != core.ActionAbandoned {
				t.Fatalf("explicit purge CLI failed: %s %s %v", out, stderr, err)
			}
			after, _ := os.ReadFile(dest)
			if string(before) != string(after) {
				t.Fatal("explicit purge changed restored file")
			}
			if _, _, err := clitest.Execute(t, NewCmd(), ref, "--all"); err == nil {
				t.Fatal("explicit purge accepted ambiguous all selection")
			}
		})
	}
}

func TestManualPurgeReportsBlockedDuringBackoff(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, asJSON := range []bool{false, true} {
		for _, dry := range []bool{false, true} {
			t.Run(fmt.Sprintf("json=%v/dry=%v", asJSON, dry), func(t *testing.T) {
				clitest.IsolateRunner(t)
				root, _ := clitest.CommandDir(t)
				path := filepath.Join(root, "keep.txt")
				clitest.WriteFixture(t, path, "keep")
				svc, err := cmdutil.Open()
				if err != nil {
					t.Fatal(err)
				}
				rows, err := svc.Bury([]string{path}, false, "")
				if err != nil || core.GraveFailed(rows) {
					t.Fatalf("bury: %+v %v", rows, err)
				}
				b := *rows[0].Burial
				clitest.WriteFixture(t, filepath.Dir(b.Grave)+".json", "{broken")
				if err := svc.Store.Update(func(st *core.State) error {
					st.Burials[0].PurgeAt = time.Now().Add(-time.Hour)
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				if _, err := svc.Purge(false, false); err != nil {
					t.Fatal(err)
				}
				st, err := svc.Store.Read()
				if err != nil || !st.Burials[0].NextPurgeAttempt.After(time.Now()) {
					t.Fatal("missing backoff", err)
				}
				b = st.Burials[0]
				var args []string
				if asJSON {
					args = append(args, "--json")
				}
				if dry {
					args = append(args, "--dry-run")
				}
				out, stderr, err := clitest.Execute(t, NewCmd(), args...)
				if err == nil || strings.Contains(out, "nothing to purge") || !strings.Contains(out+stderr, b.LastPurgeError) || !strings.Contains(out+stderr, b.NextPurgeAttempt.Format(time.RFC3339)) {
					t.Fatalf("blocked purge reported success or lost details: %s %s %v", out, stderr, err)
				}
				if asJSON {
					var got []core.GraveResult
					if err := json.Unmarshal([]byte(out), &got); err != nil || len(got) != 1 || got[0].Action != core.ActionBlocked || !core.GraveFailed(got) {
						t.Fatalf("JSON failure: %s %v", out, err)
					}
				}
				if _, err := os.Stat(b.Grave); err != nil {
					t.Fatal("blocked grave lost", err)
				}
			})
		}
	}
}

func TestPurgeDryRunVerifiesGrave(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, fault := range []string{"none", "extra-file", "outside-root", "missing-grave"} {
		for _, asJSON := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%v", fault, asJSON), func(t *testing.T) {
				clitest.IsolateRunner(t)
				root, _ := clitest.CommandDir(t)
				path := filepath.Join(root, "keep.txt")
				clitest.WriteFixture(t, path, "keep")
				svc, err := cmdutil.Open()
				if err != nil {
					t.Fatal(err)
				}
				rows, err := svc.Bury([]string{path}, false, "")
				if err != nil || len(rows) != 1 || core.GraveFailed(rows) {
					t.Fatalf("bury: %+v %v", rows, err)
				}
				b := *rows[0].Burial
				if _, err := os.Stat(filepath.Dir(b.Grave) + ".json"); !os.IsNotExist(err) {
					t.Fatalf("fixture must have no remaining manifest: %v", err)
				}
				reason := ""
				switch fault {
				case "extra-file":
					clitest.WriteFixture(t, filepath.Join(filepath.Dir(b.Grave), "stray.txt"), "unrelated")
					reason = "holds something besides"
				case "outside-root":
					if err := svc.Store.Update(func(st *core.State) error {
						st.Burials[0].Grave = path
						return nil
					}); err != nil {
						t.Fatal(err)
					}
					reason = "is not a grave in the graveyard"
				case "missing-grave":
					if err := os.Rename(filepath.Dir(b.Grave), filepath.Dir(b.Grave)+"-moved"); err != nil {
						t.Fatal(err)
					}
				}
				before, err := svc.Store.Read()
				if err != nil {
					t.Fatal(err)
				}
				history, err := svc.Store.History(100)
				if err != nil {
					t.Fatal(err)
				}
				args := []string{"--all", "--dry-run"}
				if asJSON {
					args = append(args, "--json")
				}
				out, stderr, runErr := clitest.Execute(t, NewCmd(), args...)
				wantAction := core.ActionPurged
				if fault == "missing-grave" {
					wantAction = core.ActionLost
				}
				if reason != "" {
					wantAction = core.ActionBlocked
					if runErr == nil || !strings.Contains(out+stderr, reason) || strings.Contains(out+stderr, core.ActionPurged) {
						t.Fatalf("invalid grave preview reported success: %s %s %v", out, stderr, runErr)
					}
				} else if runErr != nil {
					t.Fatal(runErr)
				}
				if asJSON {
					var got []core.GraveResult
					if err := json.Unmarshal([]byte(out), &got); err != nil || len(got) != 1 || got[0].Action != wantAction || core.GraveFailed(got) != (reason != "") {
						t.Fatalf("invalid preview receipt: %s %v", out, err)
					}
				} else if !strings.Contains(out+stderr, wantAction) {
					t.Fatalf("text preview lacks %s: %s %s", wantAction, out, stderr)
				}
				after, err := svc.Store.Read()
				if err != nil || !reflect.DeepEqual(before, after) {
					t.Fatalf("preview changed ledger: %+v %v", after, err)
				}
				afterHistory, err := svc.Store.History(100)
				if err != nil || !reflect.DeepEqual(history, afterHistory) {
					t.Fatalf("preview changed audit history: %+v %v", afterHistory, err)
				}
				kept := b.Grave
				if fault == "missing-grave" {
					kept = filepath.Join(filepath.Dir(b.Grave)+"-moved", filepath.Base(b.Grave))
				}
				data, err := os.ReadFile(kept)
				if err != nil || string(data) != "keep" {
					t.Fatalf("preview touched buried file: %q %v", data, err)
				}
				if fault == "extra-file" {
					data, err := os.ReadFile(filepath.Join(filepath.Dir(b.Grave), "stray.txt"))
					if err != nil || string(data) != "unrelated" {
						t.Fatalf("preview touched unrelated file: %q %v", data, err)
					}
				}
			})
		}
	}
}
