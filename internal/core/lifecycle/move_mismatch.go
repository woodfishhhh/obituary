package lifecycle

import (
	"errors"
	"path/filepath"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

func retainedMismatch(err error) string {
	var mismatch *corefs.MoveMismatchError
	if errors.As(err, &mismatch) && mismatch.Retained != "" {
		return mismatch.Error()
	}
	return ""
}

func retainMoveMismatch(p *preparedBurial, err error) error {
	detail := retainedMismatch(err)
	if detail == "" {
		return nil
	}
	p.retained = true
	p.burial.IdentityMismatch = detail
	p.out.Grave, p.out.GraveID = p.burial.Grave, p.burial.GraveID
	return writeMismatchManifest(p.burial)
}

func writeMismatchManifest(b Burial) error {
	// Older readers reject v4 rather than dropping its permanent purge veto.
	b.Size = 0 // size accounting belongs to the ledger, not recovery evidence
	return writeGraveManifest(filepath.Dir(b.Grave)+".json", burialManifest{Version: 4, Burial: b})
}

func (s *Service) retainRestoreMismatch(b Burial, err error) error {
	detail := retainedMismatch(err)
	if detail == "" {
		return nil
	}
	b.IdentityMismatch = detail
	manifestErr := writeMismatchManifest(b)
	ledgerErr := s.Store.UpdateGraveyard(func(st *State) error {
		if i := st.BurialIndex(b.ID); i >= 0 {
			st.Burials[i].IdentityMismatch = detail
		}
		return nil
	})
	return errors.Join(manifestErr, ledgerErr)
}
