// Package watch discovers top-level folder policy children; core owns the
// identity checks, policy decisions and ordinary restorable burial pipeline.
package watch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/core"
)

// Reconcile is the full scan used on every application tick, including --once.
// Repeated scans refresh stability and modified age while preserving manual extensions.
func Reconcile(svc *core.Service) error {
	policies, err := svc.Watches()
	if err != nil {
		return err
	}
	var errs []error
	for _, p := range policies {
		if p.Consent == core.ConsentApproved {
			errs = append(errs, reconcileFolder(svc, p))
		}
	}
	return errors.Join(errs...)
}

func reconcileFolder(svc *core.Service, p core.WatchPolicy) error {
	path, err := svc.WatchPath(p)
	if err != nil {
		return fmt.Errorf("watch %s: %w", p.Path, err)
	}
	children, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	var errs []error
	for _, child := range children {
		if err := svc.Store.Context().Err(); err != nil {
			return errors.Join(append(errs, err)...)
		}
		err := svc.RegisterWatchChild(p, filepath.Join(path, child.Name()))
		var refused *core.RefusedError
		if errors.As(err, &refused) || errors.Is(err, core.ErrNotFound) || errors.Is(err, core.ErrUnsafe) || errors.Is(err, core.ErrProtected) {
			continue // definite refusals are not retryable scan failures
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("watch %s: %w", child.Name(), err))
		}
	}
	return errors.Join(errs...)
}
