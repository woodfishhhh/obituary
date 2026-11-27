package project_test

import (
	"github.com/woodfishhhh/obituary/internal/testenv"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/project"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func TestReplayRegistersAfterGitTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	root, home := t.TempDir(), t.TempDir()
	testenv.InitRepo(t, root)
	path := filepath.Join(root, "debug.log")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Replay registers only with automatic registration on.
	svc := &lifecycle.Service{Store: ledger.Store{Home: home}, FS: winfs.FS{}, ProposalMode: lifecycle.ProposalsTiered}
	if err := svc.Store.Update(func(st *ledger.State) error {
		st.Projects = map[string]ledger.ProjectState{ledger.ProjectKey(root): {Consent: ledger.ConsentApproved}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	queued, err := svc.CaptureObservations([]string{path}, "s")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Store.DeferObservations(queued); err != nil {
		t.Fatal(err)
	}
	e := &auto.Engine{Svc: svc}
	restore := project.SetGitTimeout(time.Nanosecond) // real CommandContext deadline, no fake Git answer
	t.Cleanup(restore)
	err = e.ReplayObservations()
	restore()
	if err == nil || !svc.Store.HasDeferredObservations() {
		t.Fatalf("git timeout discarded observation: %v", err)
	}
	if err := e.ReplayObservations(); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Store.Read()
	if err != nil || svc.Store.HasDeferredObservations() || len(st.Items) != 1 || st.Items[0].Path != path {
		t.Fatalf("retry did not register: %+v %v", st.Items, err)
	}
}
