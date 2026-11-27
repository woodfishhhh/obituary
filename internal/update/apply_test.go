package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

func TestApplyReportsResultAndRestarts(t *testing.T) {
	for _, failure := range []string{"", "installer failed", "wrong installed version", "old process did not exit", "installed version changed"} {
		t.Run(failure, func(t *testing.T) {
			testApply(t, failure)
		})
	}
}

func testApply(t *testing.T, failure string) {
	t.Helper()
	home := t.TempDir()
	exe := filepath.Join(home, "scoop", "apps", "obituary", "1.3.2", "obit.exe")
	installed := filepath.Join(home, "scoop", "apps", "obituary", "current", "obit.exe")
	steps := []string{}
	before, after := "1.3.2", "1.4.0"
	if failure == "wrong installed version" {
		after = before
	}
	if failure == "installed version changed" {
		before = "1.5.0"
	}
	var ranInstall bool
	run := func(_ context.Context, out io.Writer, argv []string) error {
		if argv[0] == installed && reflect.DeepEqual(argv[1:], []string{"--version"}) {
			steps = append(steps, "verify")
			if ranInstall {
				fmt.Fprintln(out, "obit version", after)
			} else {
				fmt.Fprintln(out, "obit version", before)
			}
			return nil
		}
		if !reflect.DeepEqual(argv, []string{"scoop", "update", "obituary"}) {
			t.Fatalf("unexpected argv: %q", argv)
		}
		steps = append(steps, "install")
		ranInstall = true
		fmt.Fprintln(out, "installer output")
		if failure == "installer failed" {
			return errors.New(failure)
		}
		return nil
	}
	hooks := ApplyHooks{
		Ready: func(ApplyResult) error { steps = append(steps, "ack"); return nil },
		Wait: func() error {
			steps = append(steps, "wait")
			// The window may close as soon as the helper waits for it.
			if p, _ := ReadProgress(home); p.Phase != PhaseReady {
				t.Errorf("progress while waiting for the window: %+v", p)
			}
			if failure == "old process did not exit" {
				return errors.New(failure)
			}
			return nil
		},
		Stop: func() error { steps = append(steps, "stop"); return nil },
		Restart: func(exe string) error {
			steps = append(steps, "restart")
			if exe != installed {
				t.Errorf("restart from %q, want %q", exe, installed)
			}
			return nil
		},
		Relaunch: func(string) error {
			steps = append(steps, "relaunch")
			if result, err := ReadLastApply(home); err != nil || result == nil {
				t.Fatalf("GUI launched before result was written: %+v, %v", result, err)
			}
			return nil
		},
	}
	result, err := apply(t.Context(), home, exe, "1.3.2", "1.4.0", hooks, run)
	wantSteps := map[string]string{
		"":                          "ack wait verify stop install verify restart relaunch",
		"installer failed":          "ack wait verify stop install restart relaunch",
		"wrong installed version":   "ack wait verify stop install verify restart relaunch",
		"old process did not exit":  "ack wait restart relaunch",
		"installed version changed": "ack wait verify restart relaunch",
	}
	if strings.Join(steps, " ") != wantSteps[failure] || (failure == "") != (err == nil) {
		t.Fatalf("failure=%q, steps=%q, err=%v", failure, steps, err)
	}
	if result.At.IsZero() || result.TargetVersion != "1.4.0" || result.Success != (failure == "") || result.Success != (result.Version == "1.4.0") || result.Success != (result.Error == "") {
		t.Fatalf("result=%+v", result)
	}
	last, err := ReadLastApply(home)
	if err != nil || last == nil || *last != result {
		t.Fatalf("persisted: %+v, want %+v; %v", last, result, err)
	}
	if p, _ := ReadProgress(home); (p.Phase == PhaseDone) != (failure == "") || (failure != "" && (p.Phase != PhaseFailed || p.Error == "")) {
		t.Fatalf("final progress %+v", p)
	}
	data, err := os.ReadFile(result.LogPath)
	if err != nil || (ranInstall && !strings.Contains(string(data), "installer output")) || (failure != "" && !strings.Contains(string(data), result.Error)) {
		t.Fatalf("log: %q; %v", data, err)
	}
}

func TestApplyLockPreventsAcknowledgement(t *testing.T) {
	home := t.TempDir()
	exe := filepath.Join(home, "scoop", "apps", "obituary", "current", "obit.exe")
	unlock, ok, err := ledger.TryLockExclusive(filepath.Join(home, "update-apply.lock"))
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	defer unlock()
	hooks := ApplyHooks{Ready: func(ApplyResult) error { t.Fatal("acknowledged without apply lock"); return nil }}
	if _, err := apply(t.Context(), home, exe, "1.3.2", "1.4.0", hooks, nil); err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatal(err)
	}
}
