package app

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func TestProposalLogDeduplicatesAcrossTicksAndRestarts(t *testing.T) {
	home := t.TempDir()
	var output bytes.Buffer
	logger := log.New(&output, "", 0)
	unavailable := &auto.ProposalProjectError{Project: "project", Err: errors.New("unavailable; retry later")}
	other := errors.New("ledger failure")
	for range 3 {
		got := ProposalLogErrors(home, logger, errors.Join(unavailable, other))
		if !errors.Is(got, other) {
			t.Fatalf("unrelated diagnosis lost: %v", got)
		}
	}
	if strings.Count(output.String(), "unavailable; retry later") != 1 {
		t.Fatalf("repeated error: %s", output.String())
	}
	changed := &auto.ProposalProjectError{Project: "project", Err: errors.New("metadata unavailable; retry later")}
	if err := ProposalLogErrors(home, logger, changed); err != nil {
		t.Fatal(err)
	}
	if err := ProposalLogErrors(home, logger, nil); err != nil {
		t.Fatal(err)
	}
	if err := ProposalLogErrors(home, logger, unavailable); err != nil {
		t.Fatal(err)
	}
	if strings.Count(output.String(), "proposal project") != 4 {
		t.Fatalf("state transitions lost: %s", output.String())
	}
}

func TestProposalLogKeepsTickErrorsObservable(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "repo")
	if err := os.Mkdir(project, 0700); err != nil {
		t.Fatal(err)
	}
	svc := &core.Service{Store: core.Store{Home: filepath.Join(root, "home")}, FS: winfs.FS{}}
	_, _, _, err := svc.Propose(core.ProposalLeftovers, project, "s", []core.Candidate{{Path: filepath.Join(project, "missing"), Lifespan: "1d"}})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	for range 3 {
		_, _, err := Tick(svc)
		var projectErr *auto.ProposalProjectError
		if !errors.As(err, &projectErr) {
			t.Fatalf("tick hid diagnostic: %v", err)
		}
		if err := ProposalLogErrors(svc.Store.Home, log.New(&output, "", 0), err); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Count(output.String(), "retry later") != 1 {
		t.Fatalf("repeated tick logged: %s", output.String())
	}
	st, err := svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 || len(st.Items) != 0 {
		t.Fatalf("diagnosis changed proposal: %+v %v", st, err)
	}
}

func TestProposalLogRecoverySurvivesOtherTickErrors(t *testing.T) {
	home := t.TempDir()
	var output bytes.Buffer
	logger := log.New(&output, "", 0)
	unavailable := &auto.ProposalProjectError{Project: "a", Err: errors.New("offline")}
	if err := ProposalLogErrors(home, logger, unavailable); err != nil {
		t.Fatal(err)
	}
	other := errors.New("sweep failed")
	if err := ProposalLogErrors(home, logger, joinTickErrors(nil, other)); !errors.Is(err, other) {
		t.Fatalf("other error lost: %v", err)
	}
	if strings.Count(output.String(), `proposal project "a"`) != 2 {
		t.Fatalf("recovery masked by sweep error: %s", output.String())
	}
}

func TestProposalLogRecoveryIsPerProject(t *testing.T) {
	home := t.TempDir()
	var output bytes.Buffer
	logger := log.New(&output, "", 0)
	a := &auto.ProposalProjectError{Project: "a", Err: errors.New("offline")}
	b := &auto.ProposalProjectError{Project: "b", Err: errors.New("offline")}
	for _, err := range []error{errors.Join(a, b), b, b, errors.Join(b, a)} {
		if err := ProposalLogErrors(home, logger, err); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Count(output.String(), `proposal project "a"`) != 3 || strings.Count(output.String(), `proposal project "b"`) != 1 {
		t.Fatalf("project transitions interfere: %s", output.String())
	}
	if err := SetPaused(home, true); err != nil {
		t.Fatal(err)
	}
	if err := ProposalLogErrors(home, logger, nil); err != nil {
		t.Fatal(err)
	}
	if strings.Count(output.String(), "proposal project") != 4 {
		t.Fatalf("paused tick reset diagnosis: %s", output.String())
	}
}

func TestProposalLogCacheFailureRetainsDiagnostics(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "proposal-log.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	original := &auto.ProposalProjectError{Project: "project", Err: os.ErrPermission}
	var output bytes.Buffer
	got := ProposalLogErrors(home, log.New(&output, "", 0), original)
	if !errors.Is(got, os.ErrPermission) || !strings.Contains(got.Error(), "proposal log state") {
		t.Fatalf("cache failure swallowed error: %v", got)
	}
}
