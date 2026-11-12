package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

// From the first filesystem write through ledger commit, finish the transaction
// despite client cancellation. Preparation/move errors stay on the item; commit
// errors escape to the batch. Roll back only newly created parents before a move.
func (s *Service) restoreBurial(result *GraveResult, b Burial, state State) error {
	tx := s.WithContext(context.WithoutCancel(s.Store.Context()))
	if err := tx.cleanRecordedManifest(b, state); err != nil {
		result.Error = err.Error()
		return nil
	}
	chain, err := tx.prepareRestoreParent(b, b.Path)
	if err != nil {
		result.Error = err.Error()
		return nil
	}
	parent := chain.RestoreParent
	moved := false
	defer func() {
		if !moved {
			result.Warning = errorText(chain.Rollback())
		}
		chain.Close()
	}()
	dest, pathUnavailable, err := s.renameResurrection(b, parent)
	if err != nil {
		result.Error = err.Error()
		return nil
	}
	moved = true
	result.Action, result.Burial, result.Path = ActionResurrected, &b, dest
	detail := resurrectionDetail(b.Path, dest, pathUnavailable)
	r, err := tx.commitResurrection(b, dest, detail, &restoreIntent{Parent: parent.Identity(), Name: filepath.Base(dest)})
	result.Warning = errorText(r.Warning)
	return err
}

// The handle-relative rename is the collision authority. Every name passes
// Guard and gets a durable intent; only name collisions retry, up to 1000 suffixes.
func (s *Service) renameResurrection(b Burial, parent RestoreParent) (string, bool, error) {
	for n := 0; ; n++ {
		dest := resurrectName(b.Path, n)
		if err := s.checkResurrectionTarget(dest, parent); err != nil {
			return "", false, err
		}
		if err := writeRestoreIntent(b, parent, filepath.Base(dest)); err != nil {
			return "", false, err
		}
		actual, err := s.FS.MovePinnedInto(b.Grave, *b.Identity, parent, filepath.Base(dest))
		if err == nil {
			if actual != "" {
				return actual, false, nil
			}
			return dest, true, nil
		}
		if !errors.Is(err, os.ErrExist) || n == 1000 {
			return "", false, fmt.Errorf("#%d: %w", b.ID, err)
		}
	}
}

func (s *Service) checkResurrectionTarget(dest string, parent RestoreParent) error {
	for _, path := range []string{dest, filepath.Join(parent.Path(), filepath.Base(dest))} {
		if reason := s.Guard.ProtectedReason(path); reason != "" {
			return &RefusedError{Path: path, Reason: reason}
		}
	}
	return nil
}

func resurrectionDetail(original, dest string, pathUnavailable bool) string {
	detail := ""
	if !samePath(dest, original) {
		detail = "restored from " + original
	}
	if pathUnavailable {
		detail = strings.TrimSpace(detail + " actual restored path unavailable; expected path used")
	}
	return detail
}

func resurrectName(path string, n int) string {
	if n == 0 {
		return path
	}
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	suffix := " (resurrected)"
	if n > 1 {
		suffix = fmt.Sprintf(" (resurrected %d)", n)
	}
	// If the extension consumes the suffix budget, shorten the whole name.
	// Keep ordinary extensions, but never sacrifice a legal collision suffix.
	if len(utf16.Encode([]rune(suffix+ext))) >= 255 {
		ext = ""
	}
	stem := strings.TrimSuffix(base, ext)
	budget := 255 - len(utf16.Encode([]rune(suffix+ext)))
	units := 0
	for i, r := range stem {
		units += utf16.RuneLen(r)
		if units > budget {
			stem = stem[:i]
			break
		}
	}
	return filepath.Join(filepath.Dir(path), stem+suffix+ext)
}
