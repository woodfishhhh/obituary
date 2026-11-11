package integrate

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/woodfishhhh/obituary/internal/core"
)

// Lock each shared resource in the data directory. Keep all locks until Apply
// finishes the registration, sidecar and final receipt.
// Use core's LockFileEx wrapper (see core/lock_windows.go for the flock limits).
// Contention fails before writes so callers can replan against fresh owners.
type sharedApplyLocks struct {
	releases []func()
	locked   bool
}

func (l *sharedApplyLocks) close() {
	for i := len(l.releases) - 1; i >= 0; i-- {
		l.releases[i]()
	}
}

// The existing write seam also covers the scheduling gap between preflight
// and locking. The target invokes it again under the lock before its own I/O.
func (l *sharedApplyLocks) beforeWrite(ch Change, changes []Change) error {
	if l.locked || len(sharedApplyIntents(changes))+len(relocatedApplyIntents(changes)) == 0 {
		return nil
	}
	if beforeIntegrationWrite != nil {
		if err := beforeIntegrationWrite(ch.Path); err != nil {
			return err
		}
	}
	return l.acquire(changes)
}

func (l *sharedApplyLocks) acquire(changes []Change) error {
	if l.locked {
		return nil
	}
	intents := sharedApplyIntents(changes)
	for _, intent := range intents {
		lock, err := sharedResourceLock(intent.asset)
		if err != nil {
			return err
		}
		release, err := lockSharedResource(lock)
		if err != nil {
			return err
		}
		l.releases = append(l.releases, release)
	}
	for _, intent := range intents {
		checks := append([]Change{intent.asset.file, intent.asset.receipt}, intent.registration...)
		if m := intent.asset.migration; m != nil {
			checks = append(checks, m.file, m.receipt, m.legacyFile, m.legacyReceipt, m.legacyLock)
		}
		if lock := intent.asset.legacyLock; lock != nil {
			checks = append(checks, *lock)
		}
		for _, ch := range checks {
			if err := checkOriginal(ch); err != nil {
				return err
			}
		}
	}
	if err := l.acquireRelocated(changes); err != nil {
		return err
	}
	l.locked = true
	return nil
}

func sharedApplyIntents(changes []Change) []*sharedRegistrationPlan {
	var intents []*sharedRegistrationPlan
	seen := map[string]bool{}
	for _, ch := range changes {
		if ch.shared != nil && !seen[ch.shared.asset.asset.Path] {
			intents = append(intents, ch.shared)
			seen[ch.shared.asset.asset.Path] = true
		}
	}
	slices.SortFunc(intents, func(a, b *sharedRegistrationPlan) int {
		return strings.Compare(a.asset.asset.Path, b.asset.asset.Path)
	})
	return intents
}

func sharedResourceLock(p managedAssetPlan) (Change, error) {
	if p.migration != nil {
		return p.migration.receipt, nil
	}
	path, err := sidecarPath(p.asset.Path)
	if err != nil {
		return Change{}, err
	}
	return readIntegrationChange(Scope{Global: true}, path)
}

func lockSharedResource(ch Change) (func(), error) {
	ch.Path += ".lock"
	p, err := openIntegrationTarget(ch, true)
	if err != nil {
		return nil, err
	}
	if err := p.lockHostDirectory(); err != nil {
		p.root.Close()
		return nil, err
	}
	info, err := p.root.Lstat(p.name)
	if err == nil {
		err = ordinaryIntegrationEntry(info)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		p.root.Close()
		return nil, err
	}
	release, ok, err := core.TryLockExclusive(ch.Path)
	if err != nil || !ok {
		p.root.Close()
		return nil, fmt.Errorf("%s: shared installation busy or unavailable; run obit init again: %w", ch.Path, errors.Join(err, os.ErrPermission))
	}
	return func() { release(); p.root.Close() }, nil
}
