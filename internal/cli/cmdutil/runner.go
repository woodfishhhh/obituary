package cmdutil

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/hook"
)

var SpawnSelf = app.SpawnSelf

var RunOnce = app.RunOnce

// DeferExplorerSync coalesces presentation changes from one hook into a single
// background tick. The resident already reconciles labels on every tick; an
// absent resident gets the existing one-shot runner. Registration still commits
// synchronously, and non-hook commands retain their immediate presentation.
func DeferExplorerSync(svc *core.Service) func() {
	changed := false
	svc.AfterChange = func(*core.Service) { changed = true }
	return func() {
		if !changed || RunnerRunning(svc.Store.Home) {
			return
		}
		if err := RunOnce(svc.Store.Home); err != nil {
			hook.Logf(svc.Store.Home, "background Explorer sync: %v", err)
		}
	}
}

// runner hands a hook's slow work to background processes.
func Runner(svc *core.Service) *hook.Runner {
	return &hook.Runner{
		Tick: func() { tickInBackground(svc) },
		Finish: func(root, session string, final bool) {
			if !final && !finishDue(svc.Store.Home, root, session) {
				return
			}
			if err := SpawnSelf("hook", "finish", "--project", root, "--session", session); err != nil {
				hook.Logf(svc.Store.Home, "finish %s: %v", session, err)
			}
		},
	}
}

// finishDue debounces leftover computation: Stop fires after every turn,
// but one diff a minute per session is plenty.
func finishDue(home, root, session string) bool {
	sum := sha256.Sum256([]byte(strings.ToLower(root) + "|" + session))
	marker := filepath.Join(home, "sessions", hex.EncodeToString(sum[:8])+".finish")
	if info, err := os.Stat(marker); err == nil && time.Since(info.ModTime()) < time.Minute {
		return false
	}
	if err := os.MkdirAll(filepath.Dir(marker), 0o700); err != nil {
		return true
	}
	_ = os.WriteFile(marker, nil, 0o600)
	return true
}

// tickInBackground starts a one-shot runner when work is due and no daemon
// is there to do it, so hooks never do the slow part themselves.
func tickInBackground(svc *core.Service) {
	if app.Running(svc.Store.Home) {
		return
	}
	if svc.Store.HasDeferredObservations() {
		if err := RunOnce(svc.Store.Home); err != nil {
			hook.Logf(svc.Store.Home, "background observations: %v", err)
		}
		return
	}
	state, err := svc.Store.WithDeadline(time.Now().Add(time.Second)).Read()
	if err != nil {
		hook.Logf(svc.Store.Home, "background tick: %v", err)
		return
	}
	next, ok := state.NextDue()
	due := ok && !next.After(time.Now())
	if due || svc.Store.HasDeferredObservations() {
		if err := RunOnce(svc.Store.Home); err != nil {
			hook.Logf(svc.Store.Home, "background tick: %v", err)
		}
	} else if err := recoverInBackground(svc.Store.Home); err != nil {
		hook.Logf(svc.Store.Home, "background recovery: %v", err)
	}
}

// Manifest-only wakeups share a marker and lock across hook processes. A new
// filename, changed size or modification time bypasses the ten-minute delay.
// Due items and queued writes never consult this marker.
func recoverInBackground(home string) error {
	release, ok, err := core.TryLockExclusive(filepath.Join(home, "manifest-recovery.lock"))
	if err != nil || !ok {
		return err
	}
	defer release()
	entries, err := os.ReadDir(filepath.Join(home, "graveyard"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	hash := sha256.New()
	found := false
	for _, entry := range entries {
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(hash, "%q %d %d\n", entry.Name(), info.Size(), info.ModTime().UnixNano())
		found = true
	}
	if !found {
		return nil
	}
	marker := filepath.Join(home, "manifest-recovery.json")
	var last struct {
		At     time.Time `json:"at"`
		Digest string    `json:"digest"`
	}
	data, err := os.ReadFile(marker)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_ = json.Unmarshal(data, &last) // a missing/incomplete marker permits a retry
	digest := hex.EncodeToString(hash.Sum(nil))
	if last.Digest == digest && time.Since(last.At) < 10*time.Minute {
		return nil
	}
	if err := RunOnce(home); err != nil {
		return err // a failed spawn must remain retryable
	}
	last.At, last.Digest = time.Now(), digest
	data, err = json.Marshal(last)
	if err != nil {
		return err
	}
	return os.WriteFile(marker, data, 0o600)
}
