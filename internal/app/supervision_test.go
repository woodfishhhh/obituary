package app

import (
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestSupervisionCrashWindow(t *testing.T) {
	home, now := t.TempDir(), time.Unix(1700000000, 0)
	failure := errors.New("startup failed")
	attempt := func(at time.Time, run func() error) error {
		return managedAttempt(home, func() time.Time { return at }, run)
	}
	for i := range 3 {
		if err := attempt(now.Add(time.Duration(i)*time.Minute), func() error { return failure }); !errors.Is(err, failure) {
			t.Fatal(err)
		}
	}
	if err := attempt(now.Add(3*time.Minute), func() error { t.Fatal("fourth tray entered"); return nil }); err != nil {
		t.Fatal(err)
	}
	state, err := readSupervision(home)
	if err != nil || !state.Abandoned || len(state.Failures) != 3 {
		t.Fatal(state, err)
	}
	ran := false
	if err := attempt(now.Add(15*time.Minute), func() error { ran = true; return nil }); err != nil || !ran {
		t.Fatal(ran, err)
	}
}

func TestMissingTrayDoesNotConsumeCrashBudget(t *testing.T) {
	home := t.TempDir()
	missing := filepath.Join(home, "missing-obitd")
	err := managedAttempt(home, time.Now, func() error { return runTrayProcess(exec.Command(missing)) })
	if err == nil {
		t.Fatal("missing executable reported success")
	}
	s, e := readSupervision(home)
	if e != nil || len(s.Failures) != 0 || s.Open || !s.LaunchFailed || s.LastExitCode != nil || s.LastAttempt.IsZero() || s.Reason == "" {
		t.Fatalf("missing executable counted as a crash or lost its diagnostic: %+v %v", s, e)
	}
}

func TestThirdCrashInsideAndOutsideWindow(t *testing.T) {
	for _, tc := range []struct {
		name      string
		offset    time.Duration
		abandoned bool
	}{
		{"inside", -time.Nanosecond, true}, {"boundary", 0, false}, {"outside", time.Nanosecond, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, base := t.TempDir(), time.Unix(1700000000, 0)
			failure := errors.New("tray exited unexpectedly")
			for _, delay := range []time.Duration{0, time.Minute, 15*time.Minute + tc.offset} {
				if err := managedAttempt(home, func() time.Time { return base.Add(delay) }, func() error { return failure }); !errors.Is(err, failure) {
					t.Fatal(err)
				}
			}
			s, err := readSupervision(home)
			if err != nil || s.Abandoned != tc.abandoned {
				t.Fatalf("third exit: %+v %v", s, err)
			}
		})
	}
}

func TestSupervisionCountsUnclosedReceiptOnce(t *testing.T) {
	home := t.TempDir()
	func() {
		defer func() { _ = recover() }()
		_ = managedAttempt(home, time.Now, func() error { panic("killed") })
	}()
	for range 4 {
		if err := managedAttempt(home, time.Now, func() error {
			return managedAttempt(home, time.Now, func() error { t.Fatal("concurrent tray"); return nil })
		}); err != nil {
			t.Fatal(err)
		}
	}
	s, err := readSupervision(home)
	if err != nil || s.Open || s.Abandoned || len(s.Failures) != 1 {
		t.Fatal(s, err)
	}
}
