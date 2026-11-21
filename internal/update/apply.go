package update

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"golang.org/x/mod/semver"
)

const installTimeout = 10 * time.Minute
const versionTimeout = 10 * time.Second

type ApplyResult struct {
	TargetVersion string `json:"targetVersion"`
	LogPath       string `json:"logPath"`
}

type LastApply struct {
	ApplyResult
	At      time.Time `json:"at"`
	Version string    `json:"version,omitempty"`
	Success bool      `json:"success"`
	Error   string    `json:"error,omitempty"`
}

// ApplyHooks lets the application manage its processes without importing it here.
type ApplyHooks struct {
	Ready    func(ApplyResult) error
	Wait     func() error
	Stop     func() error
	Restart  func(executable string) error
	Relaunch func(executable string) error
	// Retire gives leftovers of a manual install update a lifespan (the
	// previous programs wait in a folder beside the new ones).
	Retire func(paths []string) error
}

type runner func(context.Context, io.Writer, []string) error

// Apply runs the cached target. Ready receives an acknowledgement only after
// this worker owns the apply lock and has opened the update log.
func Apply(ctx context.Context, home, exe, current, target string, hooks ApplyHooks) (LastApply, error) {
	return apply(ctx, home, exe, current, target, hooks, runCommand)
}

func apply(ctx context.Context, home, exe, current, target string, hooks ApplyHooks, run runner) (LastApply, error) {
	return applyFrom(ctx, home, exe, current, target, hooks, run, releaseSources)
}

// session is what one update run shares between its steps.
type session struct {
	exe, current string
	result       ApplyResult
	log          *os.File
	hooks        ApplyHooks
	run          runner
	src          sources
	progress     *tracker
}

func (s session) wait() error {
	if s.hooks.Wait == nil {
		return nil
	}
	return s.hooks.Wait()
}

func (s session) stop() error {
	if s.hooks.Stop == nil {
		return nil
	}
	if err := s.hooks.Stop(); err != nil {
		return fmt.Errorf("stop resident: %w", err)
	}
	return nil
}

// retire gives the update folder a lifespan, unless it holds the only copy of
// the previous programs. The update itself has already succeeded or rolled
// back, so a failure here is only logged.
func (s session) retire(tx *swap) {
	if tx.stranded || s.hooks.Retire == nil {
		return
	}
	if err := s.hooks.Retire([]string{tx.root}); err != nil {
		fmt.Fprintf(s.log, "The previous programs stay in %s; giving them a lifespan failed: %v\n", tx.root, err)
	}
}

func applyFrom(ctx context.Context, home, exe, current, target string, hooks ApplyHooks, run runner, sourcesFor func(version string) sources) (LastApply, error) {
	exe, channel, err := resolve(exe, current)
	if err != nil {
		return LastApply{}, err
	}
	p, err := newPlan(channel, exe, current, target)
	if err != nil {
		return LastApply{}, err
	}
	unlock, ok, err := core.TryLockExclusive(filepath.Join(home, "update-apply.lock"))
	if err != nil {
		return LastApply{}, err
	}
	if !ok {
		return LastApply{}, errors.New("an update is already in progress")
	}
	defer unlock()
	log, err := os.CreateTemp(home, "update-*.log")
	if err != nil {
		return LastApply{}, fmt.Errorf("create update log: %w", err)
	}
	result := LastApply{ApplyResult: ApplyResult{TargetVersion: p.version, LogPath: log.Name()}}
	s := session{exe: exe, current: current, result: result.ApplyResult, log: log, hooks: hooks, run: run, src: sourcesFor(p.version), progress: &tracker{home: home}}
	// Written before the handoff is acknowledged, so a window never reads the last run's.
	s.progress.phase(PhaseStarting)
	result.Version, err = p.install(ctx, s)
	result.At = time.Now().UTC()
	err = finishApply(home, &result, log, hooks, p.installedExe(exe), err)
	return result, err
}

func (p plan) install(ctx context.Context, s session) (installed string, err error) {
	exe, current, log, hooks, run := s.exe, s.current, s.log, s.hooks, s.run
	if hooks.Ready != nil {
		if err := hooks.Ready(s.result); err != nil {
			return "", fmt.Errorf("acknowledge update handoff: %w", err)
		}
	}
	if p.channel == "manual" {
		return p.installManual(ctx, s)
	}
	s.progress.phase(PhaseReady)
	if err := s.wait(); err != nil {
		return "", err
	}
	// A second invocation may have been started from an older Scoop directory
	// after the previous updater released its lock. Never reinstall or downgrade.
	if err := p.verifyBeforeInstall(ctx, exe, current, log, run); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, installTimeout)
	defer cancel()
	script, err := p.download(ctx, filepath.Dir(log.Name()), log)
	if err != nil {
		return "", err
	}
	if script != "" {
		defer func() { err = errors.Join(err, os.Remove(script)) }()
	}
	if err := s.stop(); err != nil {
		return "", err
	}
	s.progress.phase(PhaseInstalling)
	fmt.Fprintf(log, "Updating %s from %s to %s\n", p.channel, current, p.version)
	if err := run(ctx, log, p.argv(script)); err != nil {
		return "", fmt.Errorf("run %s update: %w", p.channel, err)
	}
	return p.verify(ctx, exe, current, log, run)
}

func (p plan) verifyBeforeInstall(ctx context.Context, exe, current string, log io.Writer, run runner) error {
	actual, err := installedVersion(ctx, p.installedExe(exe), log, run)
	if err != nil {
		return err
	}
	if semver.Compare(version(actual), version(current)) != 0 {
		return fmt.Errorf("installed version changed from %s to %s; open the installed version and try again", current, actual)
	}
	return nil
}

func installedVersion(ctx context.Context, exe string, log io.Writer, run runner) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, versionTimeout)
	defer cancel()
	var stdout bytes.Buffer
	if err := run(ctx, io.MultiWriter(log, &stdout), []string{exe, "--version"}); err != nil {
		return "", fmt.Errorf("verify installed version: %w", err)
	}
	actual, ok := strings.CutPrefix(strings.TrimSpace(stdout.String()), "obit version ")
	if !ok || !semver.IsValid(version(actual)) {
		return "", fmt.Errorf("cannot read installed version from %q", strings.TrimSpace(stdout.String()))
	}
	return strings.TrimPrefix(actual, "v"), nil
}

func (p plan) verify(ctx context.Context, exe, current string, log io.Writer, run runner) (string, error) {
	actual, err := installedVersion(ctx, p.installedExe(exe), log, run)
	if err != nil {
		return "", err
	}
	if !Newer(current, actual) || semver.Compare(version(actual), version(p.version)) < 0 {
		return "", fmt.Errorf("expected installed version %s or newer, got %q", p.version, actual)
	}
	return actual, nil
}

func finishApply(home string, result *LastApply, log *os.File, hooks ApplyHooks, exe string, cause error) error {
	if hooks.Restart != nil {
		cause = errors.Join(cause, hooks.Restart(exe))
	}
	if cause != nil {
		fmt.Fprintln(log, cause)
	}
	if err := log.Close(); err != nil {
		cause = errors.Join(cause, err)
	}
	// Save before relaunch so the About page can read the completed update.
	stateErr := saveLastApply(home, result, cause)
	if hooks.Relaunch != nil {
		if err := hooks.Relaunch(exe); err != nil {
			cause = errors.Join(cause, err)
			stateErr = errors.Join(stateErr, saveLastApply(home, result, cause))
		}
	}
	err := errors.Join(cause, stateErr)
	final := Progress{Phase: PhaseDone}
	if err != nil {
		final = Progress{Phase: PhaseFailed, Error: err.Error()}
	}
	(&tracker{home: home}).set(final)
	if err != nil {
		return fmt.Errorf("%w (update log: %s)", err, result.LogPath)
	}
	return nil
}

func saveLastApply(home string, result *LastApply, cause error) error {
	result.Success = cause == nil
	if cause != nil {
		result.Error = cause.Error()
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return core.WriteMetadata(filepath.Join(home, "update-apply.json"), append(data, '\n'))
}

func ReadLastApply(home string) (*LastApply, error) {
	data, err := core.ReadMetadata(filepath.Join(home, "update-apply.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result LastApply
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("read last update: %w", err)
	}
	return &result, nil
}
