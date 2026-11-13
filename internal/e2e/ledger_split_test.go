//go:build windows

package e2e_test

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/core"
)

func TestHookProcessesInterleaveGraveyardCommits(t *testing.T) {
	if testing.Short() {
		t.Skip("native hooks and terminal commands in an isolated Git repository; core covers hot/joint transactions in short mode")
	}
	root, store := commandRepo(t)
	svc, err := app.Open()
	if err != nil {
		t.Fatal(err)
	}
	svc.AfterChange = nil
	paths := []string{filepath.Join(root, "restore.txt"), filepath.Join(root, "purge.txt")}
	for _, path := range paths {
		writeFixture(t, path, "owned fixture")
	}
	graves, err := svc.Bury(paths, false, "")
	if err != nil || core.GraveFailed(graves) {
		t.Fatal(graves, err)
	}
	if err := store.UpdateHot(func(st *core.State) error {
		st.Projects = map[string]core.ProjectState{core.ProjectKey(root): {Consent: core.ConsentApproved}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	hookPaths := make([]string, 4)
	for i := range hookPaths {
		hookPaths[i] = filepath.Join(root, fmt.Sprintf("debug-interleaved-%d.log", i))
		writeFixture(t, hookPaths[i], "hook artifact")
	}
	hooks, outputs := startQueuedHooks(t, root, hookPaths)
	var terminals []*exec.Cmd
	var terminalOutputs []*bytes.Buffer
	for i, args := range [][]string{{"resurrect", fmt.Sprint(graves[0].Burial.ID)}, {"purge", fmt.Sprint(graves[1].Burial.ID), "--yes"}} {
		cmd := exec.Command(binaryPath(t, "obit"), args...)
		output := new(bytes.Buffer)
		cmd.Stdout, cmd.Stderr = output, output
		if err := cmd.Start(); err != nil {
			t.Fatal(i, err)
		}
		t.Cleanup(func() {
			if cmd.ProcessState == nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		})
		terminals, terminalOutputs = append(terminals, cmd), append(terminalOutputs, output)
	}
	for i, cmd := range hooks {
		if err := cmd.Wait(); err != nil {
			t.Fatal("hook", i, outputs[i].String(), err)
		}
	}
	for i, cmd := range terminals {
		if err := cmd.Wait(); err != nil {
			t.Fatal("terminal", i, terminalOutputs[i].String(), err)
		}
	}
	if _, _, err := app.Tick(svc); err != nil {
		t.Fatal(err)
	}
	waitForQueueItems(t, store, len(hookPaths))
	st, err := store.ReadSnapshot()
	if err != nil || len(st.Burials) != 0 || len(st.RestoreCleanups) != 0 || st.Stats.Resurrected != 1 || st.Stats.Purged != 1 {
		t.Fatal("interleaved hook and graveyard commits lost terminal receipts", st, err)
	}
	for _, item := range st.Items {
		if item.Rev != 1 {
			t.Fatal("hook request was counted twice", item)
		}
	}
}
