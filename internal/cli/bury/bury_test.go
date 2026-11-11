//go:build windows

package bury

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	addcmd "github.com/woodfishhhh/obituary/internal/cli/add"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	mcpcmd "github.com/woodfishhhh/obituary/internal/cli/mcp"
	"github.com/woodfishhhh/obituary/internal/core"
	"golang.org/x/sys/windows"
)

func TestBuryRecoveryReceiptLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a large batch of recovery manifests to exhaust the receipt budget")
	}
	clitest.IsolateRunner(t)
	repo, _ := clitest.CommandDir(t)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	// Stage the on-disk state after 201 committed restores with unfinished
	// cleanup. Every object, grave and parent has a real Windows identity;
	// recovery still verifies each one. Avoid 402 unrelated durable commands.
	parent, err := svc.FS.IdentifyFollowing(repo)
	if err != nil {
		t.Fatal(err)
	}
	var receipts []map[string]any
	for i := range 201 {
		path := filepath.Join(repo, fmt.Sprintf("receipt-%d.txt", i))
		clitest.WriteFixture(t, path, "restore")
		id, err := svc.FS.Identify(path)
		if err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(svc.Store.Home, "graveyard", fmt.Sprintf("%d-20261107T120000Z-abcdef", i+1))
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		graveID, err := svc.FS.Identify(dir)
		if err != nil {
			t.Fatal(err)
		}
		b := core.Burial{ID: int64(i + 1), Path: path, Grave: filepath.Join(dir, filepath.Base(path)), Identity: &id, GraveID: &graveID}
		receipts = append(receipts, map[string]any{"kind": core.ActionResurrected, "burial": b, "path": path, "resurrect": map[string]any{"parent": parent, "name": filepath.Base(path)}})
	}
	encoded, err := json.Marshal(receipts)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Store.Update(func(st *core.State) error { st.NextID = 202; return json.Unmarshal(encoded, &st.RestoreCleanups) }); err != nil {
		t.Fatal(err)
	}
	clitest.WriteFixture(t, filepath.Join(svc.Store.Home, "audit.jsonl"), "")
	p, _ := windows.UTF16PtrFromString(filepath.Join(svc.Store.Home, "audit.jsonl"))
	h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	previous := cmdutil.Open
	cmdutil.Open = func() (*core.Service, error) { return svc, nil }
	t.Cleanup(func() { cmdutil.Open = previous })
	for _, mode := range []string{"mcp", "json"} {
		path := filepath.Join(repo, "new-"+mode+".txt")
		clitest.WriteFixture(t, path, "new burial")
		var results []core.GraveResult
		if mode == "mcp" {
			session := clitest.ConnectMCP(t, mcpcmd.NewServer(svc, "0.1.0-dev"))
			var data struct{ Results []core.GraveResult }
			r := clitest.CallMCP(t, session, "obit_bury", map[string]any{"paths": []string{path}}, false, &data)
			encoded, err := json.Marshal(r)
			if err != nil || len(encoded) > 1<<20 {
				t.Fatalf("MCP burial reply oversized: %d %v", len(encoded), err)
			}
			results = data.Results
		} else {
			out, _, err := clitest.Execute(t, NewCmd(), "--json", path)
			if err != nil || json.Unmarshal([]byte(out), &results) != nil {
				t.Fatalf("CLI burial reply: %s %v", out, err)
			}
		}
		if len(results) != 21 || results[0].Burial == nil || results[0].Burial.Path != path || results[0].Action != core.ActionBuried || results[0].RecoveryCount != 201 || results[0].RecoveryOmitted != 181 || !strings.Contains(results[0].Warning, "181") || !strings.Contains(results[0].Warning, "obit log --lines ") {
			t.Fatalf("burial recovery receipts unbounded or current input lost: mode=%s rows=%d first=%+v", mode, len(results), results[0])
		}
	}
}

func TestBuryPrintsRecoveredResurrection(t *testing.T) {
	svc, paths, unlock := clitest.RestoreReceiptFixture(t)
	if _, err := svc.Resurrect(paths); err == nil {
		t.Fatal("fixture did not interrupt resurrection")
	}
	unlock()
	st, err := svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Dir(st.Burials[0].Grave) + ".json"
	name, err := windows.UTF16PtrFromString(manifest)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	old := cmdutil.Open
	cmdutil.Open = func() (*core.Service, error) { return svc, nil }
	t.Cleanup(func() { cmdutil.Open = old })
	out, stderr, _ := clitest.Execute(t, NewCmd(), filepath.Join(filepath.Dir(paths[0]), "absent"))
	if !strings.Contains(out, "resurrected") || !strings.Contains(out, "recovered") || strings.Contains(out, "restorable until") || !strings.Contains(stderr, "warning:") {
		t.Fatalf("bury misreported recovered resurrection: stdout=%s stderr=%s", out, stderr)
	}
}

func TestBuryReplacementDoesNotForgetRenamedObject(t *testing.T) {
	clitest.IsolateRunner(t)
	repo, store := clitest.CommandDir(t)
	path := filepath.Join(repo, "debug.log")
	clitest.WriteFixture(t, path, "old")
	if _, _, err := clitest.Execute(t, addcmd.NewCmd(), path, "30d"); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, filepath.Join(repo, "keep.txt")); err != nil {
		t.Fatal(err)
	}
	clitest.WriteFixture(t, path, "new")
	if _, _, err := clitest.Execute(t, NewCmd(), path); err != nil {
		t.Fatal(err)
	}
	st, err := store.Read()
	if err != nil || len(st.Items) != 1 || len(st.Burials) != 1 || st.Items[0].ID == st.Burials[0].ID {
		t.Fatalf("burial consumed another object's record: %+v, %v", st, err)
	}
}

func TestBuryCommandReportsFailureWithoutRegistration(t *testing.T) {
	for _, asJSON := range []bool{false, true} {
		t.Run(fmt.Sprint(asJSON), func(t *testing.T) {
			starts := clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			path := filepath.Join(repo, "blocked.log")
			clitest.WriteFixture(t, path, "keep")
			svc, err := cmdutil.Open()
			if err != nil {
				t.Fatal(err)
			}
			svc.FS = failMoveFS{svc.FS}
			original := cmdutil.Open
			cmdutil.Open = func() (*core.Service, error) { return svc, nil }
			t.Cleanup(func() { cmdutil.Open = original })
			args := []string{path}
			if asJSON {
				args = append(args, "--json")
			}
			out, stderr, err := clitest.Execute(t, NewCmd(), args...)
			st, readErr := store.Read()
			if err == nil || readErr != nil || len(st.Items) != 0 || len(*starts) != 0 {
				t.Fatalf("failed explicit burial must not register or start the runner: %s %s, %v; %+v; starts=%v", out, stderr, err, st.Items, *starts)
			}
			if !strings.Contains(out, "retry") || !strings.Contains(out, "sharing violation") {
				t.Fatalf("missing retry receipt: %s %s", out, stderr)
			}
			if asJSON {
				var rows []struct {
					Action string
					Item   *core.Item
				}
				if json.Unmarshal([]byte(out), &rows) != nil || len(rows) != 1 || rows[0].Item != nil || stderr != "" {
					t.Fatalf("JSON must report failure without a new item: %s %s", out, stderr)
				}
			}
		})
	}
}

func TestBuryCommandReportsPartialSuccess(t *testing.T) {
	for _, asJSON := range []bool{false, true} {
		t.Run(fmt.Sprint(asJSON), func(t *testing.T) {
			starts := clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			// The parent operand starts after its child's batch has committed.
			paths := []string{filepath.Join(repo, "parent", "a.log"), filepath.Join(repo, "parent"), filepath.Join(repo, "c.log")}
			for _, path := range []string{paths[0], paths[2]} {
				clitest.WriteFixture(t, path, "keep")
			}
			svc, err := cmdutil.Open()
			if err != nil {
				t.Fatal(err)
			}
			blocked := filepath.Join(store.Home, "store.json.tmp")
			injected := false
			svc.FS = approvalFaultFS{FS: svc.FS, beforeIdentify: func(path string) {
				if strings.EqualFold(path, paths[1]) && !injected {
					injected = true
					if err := os.Remove(blocked); err != nil && !errors.Is(err, os.ErrNotExist) {
						t.Fatal(err)
					}
					if err := os.Mkdir(blocked, 0o700); err != nil {
						t.Fatal(err)
					}
				}
			}}
			original := cmdutil.Open
			cmdutil.Open = func() (*core.Service, error) { return svc, nil }
			t.Cleanup(func() { cmdutil.Open = original })
			args := append([]string(nil), paths...)
			if asJSON {
				args = append(args, "--json")
			}
			out, stderr, err := clitest.Execute(t, NewCmd(), args...)
			if err == nil || len(*starts) != 1 {
				t.Fatalf("partial failure must return error and serve the completed burial: %s %s, %v, %v", out, stderr, err, *starts)
			}
			if asJSON {
				var rows []core.GraveResult
				if json.Unmarshal([]byte(out), &rows) != nil || len(rows) != 3 || rows[0].Action != core.ActionBuried || rows[0].Burial == nil || rows[1].Error == "" || rows[2].Action != "not executed" {
					t.Fatalf("partial JSON receipt missing: %s %s", out, stderr)
				}
				for i := range rows {
					if rows[i].Ref != paths[i] {
						t.Fatalf("missing reference: %+v", rows)
					}
				}
			} else {
				for _, want := range append(paths, "buried", "not executed") {
					if !strings.Contains(out, want) {
						t.Errorf("missing %q from partial text receipt: %s %s", want, out, stderr)
					}
				}
			}
		})
	}
}

func TestBuryPendingReceiptCarriesLocationAndRetry(t *testing.T) {
	for _, format := range []string{"text", "json", "mcp", "mcp-long"} {
		t.Run(format, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			svc, err := cmdutil.Open()
			if err != nil {
				t.Fatal(err)
			}
			if format == "mcp-long" {
				store.Home = filepath.Join(store.Home, strings.Repeat("长", 180))
				svc.Store = store
			}
			path := filepath.Join(repo, "payload.txt")
			clitest.WriteFixture(t, path, "keep")
			var held windows.Handle
			svc.FS = lockedBurialFS{FS: svc.FS, lock: func() {
				p, _ := windows.UTF16PtrFromString(filepath.Join(store.Home, "store.json"))
				var err error
				held, err = windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
				if err != nil {
					t.Fatal(err)
				}
			}}
			t.Cleanup(func() {
				if held != 0 {
					windows.CloseHandle(held)
				}
			})
			previous := cmdutil.Open
			cmdutil.Open = func() (*core.Service, error) { return svc, nil }
			t.Cleanup(func() { cmdutil.Open = previous })
			var rows []struct {
				Action, Grave, Detail string
				Burial                *core.Burial
			}
			var out string
			if strings.HasPrefix(format, "mcp") {
				var reply struct{ Results json.RawMessage }
				clitest.CallMCP(t, clitest.ConnectMCP(t, mcpcmd.NewServer(svc, "0.1.0-dev")), "obit_bury", map[string]any{"paths": []string{path}}, true, &reply)
				out = string(reply.Results)
			} else {
				args := []string{path}
				if format == "json" {
					args = append(args, "--json")
				}
				out, _, err = clitest.Execute(t, NewCmd(), args...)
				if err == nil {
					t.Fatal("pending burial reported success")
				}
			}
			pending, err := svc.PendingGraves()
			if err != nil || len(pending) != 1 || pending[0].Burial == nil {
				t.Fatalf("pending fixture: %+v %v", pending, err)
			}
			grave := pending[0].Burial.Grave
			if !strings.Contains(out, core.ActionRecoveryPending) || !strings.Contains(out, "rerun obit reap") {
				t.Fatalf("pending burial receipt omitted location or retry: %s", out)
			}
			if format != "text" {
				if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) != 1 || format != "mcp-long" && rows[0].Grave != grave || rows[0].Burial != nil || !strings.Contains(rows[0].Detail, "rerun obit reap") {
					t.Fatalf("pending burial forged commit or lost location: %s %v", out, err)
				}
			} else if !strings.Contains(out, grave) {
				t.Fatalf("pending burial text lost grave: %s", out)
			}
			if data, err := os.ReadFile(grave); err != nil || string(data) != "keep" {
				t.Fatal("pending burial receipt cannot locate payload", err)
			}
		})
	}
}
