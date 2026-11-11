//go:build windows

package resurrect

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	addcmd "github.com/woodfishhhh/obituary/internal/cli/add"
	burycmd "github.com/woodfishhhh/obituary/internal/cli/bury"
	cancelcmd "github.com/woodfishhhh/obituary/internal/cli/cancel"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	extendcmd "github.com/woodfishhhh/obituary/internal/cli/extend"
	graveyardcmd "github.com/woodfishhhh/obituary/internal/cli/graveyard"
	purgecmd "github.com/woodfishhhh/obituary/internal/cli/purge"
	removecmd "github.com/woodfishhhh/obituary/internal/cli/remove"
	sweepcmd "github.com/woodfishhhh/obituary/internal/cli/sweep"
	"github.com/woodfishhhh/obituary/internal/core"
	"golang.org/x/sys/windows"
)

func TestResurrectLastCommandBatch(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, command := range []string{"bury", "rm"} {
		t.Run(command, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			t.Setenv("OBIT_SESSION_ID", "")
			paths := []string{filepath.Join(repo, "a.txt"), filepath.Join(repo, "sub", "b.txt")}
			for _, path := range paths {
				clitest.WriteFixture(t, path, "keep")
			}
			cmd := burycmd.NewCmd()
			if command == "rm" {
				cmd = removecmd.NewCmd(false)
			}
			if out, stderr, err := clitest.Execute(t, cmd, paths...); err != nil {
				t.Fatal(out, stderr, err)
			}
			st, err := store.ReadSnapshot()
			if err != nil || len(st.Burials) != 2 || st.Burials[0].Operation == nil || st.Burials[1].Operation == nil || st.Burials[0].Operation.ID != st.Burials[1].Operation.ID {
				t.Fatalf("one command split into different batches: %+v %v", st.Burials, err)
			}
			clitest.WriteFixture(t, paths[0], "replacement")
			out, stderr, err := clitest.Execute(t, NewCmd(), "--last", "--json")
			var rows []core.GraveResult
			if err != nil || json.Unmarshal([]byte(out), &rows) != nil || len(rows) != 2 || core.GraveFailed(rows) {
				t.Fatal(out, stderr, err)
			}
			for _, r := range rows {
				if got, err := os.ReadFile(r.Path); err != nil || string(got) != "keep" {
					t.Fatal(r, err)
				}
			}
			if got, err := os.ReadFile(paths[0]); err != nil || string(got) != "replacement" {
				t.Fatal("existing file overwritten", err)
			}
			if _, _, err := clitest.Execute(t, NewCmd(), "--last"); err == nil || !strings.Contains(err.Error(), "obit graveyard") {
				t.Fatal("empty scope guessed", err)
			}
		})
	}
}

func TestResurrectLastSessionAndPartialFailure(t *testing.T) {
	clitest.IsolateRunner(t)
	repo, store := clitest.CommandDir(t)
	t.Setenv("OBIT_SESSION_ID", "mine")
	paths := []string{filepath.Join(repo, "a"), filepath.Join(repo, "b")}
	for _, path := range paths {
		clitest.WriteFixture(t, path, "keep")
	}
	if _, _, err := clitest.Execute(t, burycmd.NewCmd(), paths...); err != nil {
		t.Fatal(err)
	}
	st, err := store.ReadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	clitest.WriteFixture(t, filepath.Join(filepath.Dir(st.Burials[0].Grave), "foreign"), "do not touch")
	t.Setenv("OBIT_SESSION_ID", "other")
	foreign := filepath.Join(repo, "foreign-session")
	clitest.WriteFixture(t, foreign, "other")
	if _, _, err := clitest.Execute(t, burycmd.NewCmd(), foreign); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OBIT_SESSION_ID", "mine")
	out, stderr, err := clitest.Execute(t, NewCmd(), "--last", "--json")
	var rows []core.GraveResult
	if err == nil || json.Unmarshal([]byte(out), &rows) != nil || len(rows) != 2 || rows[0].Error == "" || rows[1].Action != core.ActionResurrected {
		t.Fatalf("partial batch receipts lost: %s %s %v", out, stderr, err)
	}
	if _, err := os.Stat(foreign); !os.IsNotExist(err) {
		t.Fatal("foreign session resurrected", err)
	}
	if _, _, err := clitest.Execute(t, NewCmd(), "--last", "1"); err == nil {
		t.Fatal("--last accepted explicit IDs")
	}
}

func TestResurrectLastAgentSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, mode := range []string{"claude", "codex", "terminal", "explicit"} {
		t.Run(mode, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			svc, err := cmdutil.Open()
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now()
			svc.Now = func() time.Time { return now }
			previous := cmdutil.Open
			cmdutil.Open = func() (*core.Service, error) { return svc, nil }
			t.Cleanup(func() { cmdutil.Open = previous })
			variable, prefix := "CLAUDE_CODE_SESSION_ID", "claude:"
			if mode == "codex" {
				variable, prefix = "CODEX_THREAD_ID", "codex:"
			}
			for _, agent := range []string{"a", "b"} {
				t.Setenv(variable, agent)
				if mode == "explicit" {
					t.Setenv("OBIT_SESSION_ID", "explicit-"+agent)
					prefix = "explicit-"
				}
				path := filepath.Join(repo, agent)
				clitest.WriteFixture(t, path, agent)
				if out, stderr, err := clitest.Execute(t, removecmd.NewCmd(false), path); err != nil {
					t.Fatal(out, stderr, err)
				}
				now = now.Add(time.Second)
			}
			st, err := store.ReadSnapshot()
			if err != nil || len(st.Burials) != 2 {
				t.Fatal(st, err)
			}
			for i, agent := range []string{"a", "b"} {
				if st.Burials[i].Operation.Session != prefix+agent {
					t.Errorf("session not recorded: %+v", st.Burials[i].Operation)
				}
			}
			order := []string{"a", "b"}
			if mode == "terminal" {
				order = []string{"b", "a"}
			}
			for i, agent := range order {
				t.Setenv(variable, agent)
				if mode == "terminal" {
					t.Setenv(variable, "")
				}
				if mode == "explicit" {
					t.Setenv("OBIT_SESSION_ID", "explicit-"+agent)
					t.Setenv(variable, "other")
				}
				out, stderr, err := clitest.Execute(t, NewCmd(), "--last", "--json")
				var rows []core.GraveResult
				if err != nil || json.Unmarshal([]byte(out), &rows) != nil || len(rows) != 1 || rows[0].Path != filepath.Join(repo, agent) || rows[0].Action != core.ActionResurrected {
					t.Fatalf("wrong session restored: %s %s %v", out, stderr, err)
				}
				if data, err := os.ReadFile(rows[0].Path); err != nil || string(data) != agent {
					t.Fatal("restored content changed", err)
				}
				if i == 0 {
					if _, err := os.Stat(filepath.Join(repo, order[1])); !os.IsNotExist(err) {
						t.Fatal("unselected session restored", err)
					}
				}
			}
		})
	}
}

func TestResurrectLastReportsRecoveredBatch(t *testing.T) {
	for _, asJSON := range []bool{false, true} {
		t.Run(fmt.Sprint(asJSON), func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			t.Setenv("OBIT_SESSION_ID", "mine")
			path := filepath.Join(repo, "payload")
			clitest.WriteFixture(t, path, "keep")
			if _, _, err := clitest.Execute(t, burycmd.NewCmd(), path); err != nil {
				t.Fatal(err)
			}
			svc, err := cmdutil.Open()
			if err != nil {
				t.Fatal(err)
			}
			blocked := filepath.Join(store.Home, "store.json.tmp")
			svc.FS = &failBurialCommitFS{FS: svc.FS, blocked: blocked}
			if rows, err := svc.Resurrect([]string{path}); err == nil || len(rows) != 1 || rows[0].Path != path {
				t.Fatalf("fixture did not fail the post-move commit: %+v %v", rows, err)
			}
			if err := os.Remove(blocked); err != nil {
				t.Fatal(err)
			}
			args := []string{"--last"}
			if asJSON {
				args = append(args, "--json")
			}
			out, stderr, err := clitest.Execute(t, NewCmd(), args...)
			if err != nil {
				t.Fatalf("reconciled batch reported as missing: %s %s %v", out, stderr, err)
			}
			if asJSON {
				var rows []core.GraveResult
				if json.Unmarshal([]byte(out), &rows) != nil || len(rows) != 1 || rows[0].Action != core.ActionResurrected || rows[0].Path != path {
					t.Fatalf("reconciled receipt lost: %s", out)
				}
			} else if !strings.Contains(out, "resurrected") || !strings.Contains(out, path) {
				t.Fatalf("reconciled receipt lost: %s", out)
			}
			st, err := store.ReadSnapshot()
			if err != nil || len(st.Burials) != 0 || len(st.RestoreCleanups) != 0 {
				t.Fatal("recovery left ledger entries", st, err)
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
				t.Fatal("recovery changed restored content", err)
			}
		})
	}
}

func TestResurrectCommandPrefersLatestCommittedBurial(t *testing.T) {
	svc, older, newer := clitest.CommittedRetryFixture(t)
	out, stderr, err := clitest.Execute(t, NewCmd(), newer.Path, "--json")
	if err != nil {
		t.Fatalf("committed retry selected older active burial: %s %s %v", out, stderr, err)
	}
	var rows []core.GraveResult
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatal(err)
	}
	clitest.AssertCommittedRetry(t, svc, older, newer, rows)
}

func TestResurrectCommandRetriesUnverifiableSelection(t *testing.T) {
	for _, mode := range []string{"manifest", "scan"} {
		t.Run(mode, func(t *testing.T) {
			svc, older, newer := clitest.BurialRetryFixture(t, mode)
			previous := cmdutil.Open
			cmdutil.Open = func() (*core.Service, error) { return svc, nil }
			t.Cleanup(func() { cmdutil.Open = previous })
			out, stderr, err := clitest.Execute(t, NewCmd(), newer.Path, "--json")
			clitest.AssertRetryPreservesBurials(t, older, newer, mode, fmt.Sprint(out, stderr, err), err != nil)
		})
	}
}

func TestResurrectFailureReportsSelectedIdentity(t *testing.T) {
	clitest.IsolateRunner(t)
	repo, _ := clitest.CommandDir(t)
	path := filepath.Join(repo, "payload.txt")
	clitest.WriteFixture(t, path, "payload")
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := svc.Bury([]string{path}, false, "")
	if err != nil || core.GraveFailed(rows) {
		t.Fatal(rows, err)
	}
	b := rows[0].Burial
	clitest.WriteFixture(t, filepath.Join(filepath.Dir(b.Grave), "foreign.txt"), "keep")
	out, stderr, err := clitest.Execute(t, NewCmd(), fmt.Sprint(b.ID))
	if err == nil || !strings.Contains(out+stderr, fmt.Sprintf("id=%d", b.ID)) || !strings.Contains(out+stderr, "path=") || !strings.Contains(out+stderr, "payload.txt") {
		t.Fatalf("selected resurrection failure lost ID or original path: %s %s %v", out, stderr, err)
	}
}

func TestCommandsResolveShortWorkingDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, command := range []string{"resurrect", "purge", "spare", "extend"} {
		t.Run(command, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			path := filepath.Join(repo, "x")
			clitest.WriteFixture(t, path, "keep")
			p, _ := windows.UTF16PtrFromString(repo)
			buf := make([]uint16, 32768)
			n, err := windows.GetShortPathName(p, &buf[0], uint32(len(buf)))
			if err != nil {
				t.Fatal(err)
			}
			short := windows.UTF16ToString(buf[:n])
			if !strings.Contains(short, "~") {
				t.Skip("volume has no short-name aliases")
			}
			t.Chdir(short)
			if command == "spare" || command == "extend" {
				if _, _, err := clitest.Execute(t, addcmd.NewCmd(), "x", "1d"); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else if _, _, err := clitest.Execute(t, burycmd.NewCmd(), "x"); err != nil {
				t.Fatal(err)
			}
			var cmd *cobra.Command
			args := []string{"x", "--json"}
			switch command {
			case "resurrect":
				cmd = NewCmd()
			case "purge":
				cmd = purgecmd.NewCmd()
			case "spare":
				cmd = cancelcmd.NewCmd()
			case "extend":
				cmd = extendcmd.NewCmd()
				args = []string{"x", "1d", "--json"}
			}
			out, stderr, err := clitest.Execute(t, cmd, args...)
			if err != nil {
				t.Fatalf("short working-directory reference failed: %s %s %v", out, stderr, err)
			}
			st, err := store.ReadSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			if command == "extend" && (len(st.Items) != 1 || strings.Contains(st.Items[0].Path, "~")) {
				t.Fatalf("short reference wrote alias back into ledger: %+v", st.Items)
			}
		})
	}
}

func TestResurrectCommandReportsPartialCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, asJSON := range []bool{false, true} {
		t.Run(fmt.Sprint(asJSON), func(t *testing.T) {
			svc, paths, unlock := clitest.RestoreReceiptFixture(t)
			original := cmdutil.Open
			cmdutil.Open = func() (*core.Service, error) { return svc, nil }
			t.Cleanup(func() { cmdutil.Open = original })
			args := append([]string(nil), paths...)
			if asJSON {
				args = append(args, "--json")
			}
			out, stderr, err := clitest.Execute(t, NewCmd(), args...)
			actual := filepath.Join(filepath.Dir(paths[1]), "b (resurrected).txt")
			if err == nil {
				t.Fatal("failed restore commit reported success")
			}
			if asJSON {
				var rows []core.GraveResult
				if json.Unmarshal([]byte(out), &rows) != nil || len(rows) != 3 || rows[0].Path != paths[0] || rows[1].Path != actual || rows[1].Error == "" || rows[2].Action != core.ActionNotExecuted {
					t.Fatalf("restore partial receipt lost completed paths: %s %s", out, stderr)
				}
			} else if !strings.Contains(out, paths[0]) || !strings.Contains(out, actual) || !strings.Contains(out+stderr, paths[2]) {
				t.Fatalf("restore partial receipt lost completed paths: %s %s", out, stderr)
			}
			unlock()
			out, _, err = clitest.Execute(t, graveyardcmd.NewCmd(), "--json")
			var view graveyardcmd.GraveyardListing
			if err != nil || json.Unmarshal([]byte(out), &view) != nil || len(view.Burials) != 1 || view.Burials[0].Path != paths[2] || len(view.Restored) != 1 || view.Restored[0].Path != actual {
				t.Fatalf("graveyard did not reconcile interrupted resurrection: %s %v", out, err)
			}
		})
	}
}

func TestCommandsRecoverFailedBurialCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, operation := range []string{"sweep", "bury"} {
		t.Run(operation, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandRepo(t)
			path := filepath.Join(repo, "debug.log")
			clitest.WriteFixture(t, path, "restorable")
			svc, err := cmdutil.Open()
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().Add(-2 * time.Hour)
			svc.Now = func() time.Time { return now }
			added, err := svc.Add([]string{path}, core.AddOptions{ExpiresAt: now.Add(time.Hour), Source: "rule", Project: repo})
			if err != nil || core.Failed(added) {
				t.Fatalf("add: %+v, %v", added, err)
			}
			now = now.Add(time.Hour)
			blocked := filepath.Join(store.Home, "store.json.tmp")
			svc.FS = &failBurialCommitFS{FS: svc.FS, blocked: blocked}
			if operation == "sweep" {
				_, err = svc.Sweep(false)
			} else {
				_, err = svc.Bury([]string{path}, false, "")
			}
			if err == nil {
				t.Fatal("commit failure was not injected")
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("expected moved object: %v", err)
			}
			if err := os.Remove(blocked); err != nil {
				t.Fatal(err)
			}
			if out, stderr, err := clitest.Execute(t, sweepcmd.NewCmd(), "--json"); err != nil {
				t.Fatalf("recovery sweep: %s %s, %v", out, stderr, err)
			}
			out, _, err := clitest.Execute(t, graveyardcmd.NewCmd(), "--json")
			var listing graveyardcmd.GraveyardListing
			if err != nil || json.Unmarshal([]byte(out), &listing) != nil || len(listing.Burials) != 1 || listing.Burials[0].ID != added[0].Item.ID || listing.Burials[0].Path != path {
				t.Fatalf("graveyard lost recovery index: %s, %v", out, err)
			}
			out, stderr, err := clitest.Execute(t, NewCmd(), fmt.Sprint(listing.Burials[0].ID))
			if err != nil || !strings.Contains(out, path) {
				t.Fatalf("resurrect: %s %s, %v", out, stderr, err)
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != "restorable" {
				t.Fatalf("restored content: %q, %v", data, err)
			}
		})
	}
}
