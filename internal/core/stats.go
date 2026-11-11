package core

import (
	"io/fs"
	"path/filepath"
	"time"
)

func treeSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

// sizeRecoveredBurials fills in approximate sizes after recovery's transaction.
func (s *Service) sizeRecoveredBurials(burials []Burial) error {
	if len(burials) == 0 {
		return nil
	}
	for i := range burials {
		burials[i].Size = treeSize(burials[i].Grave)
	}
	return s.saveBurialSizes(burials)
}

func (s *Service) saveBurialSizes(burials []Burial) error {
	return s.Store.Update(func(st *State) error {
		for _, b := range burials {
			idx := st.BurialIndex(b.ID)
			if idx < 0 || st.Burials[idx].GraveID == nil || *st.Burials[idx].GraveID != *b.GraveID {
				continue // resurrected or purged while measuring
			}
			st.Stats.BuriedBytes += b.Size - st.Burials[idx].Size
			st.Burials[idx].Size = b.Size
		}
		return nil
	})
}

// Summary is what `obit stats` shows.
type Summary struct {
	Tracked    int        `json:"tracked"`
	NextDue    *time.Time `json:"nextDue,omitempty"`
	Graves     int        `json:"graves"`
	GraveBytes int64      `json:"graveBytes"`
	NextPurge  *time.Time `json:"nextPurge,omitempty"`
	Lifetime   Stats      `json:"lifetime"`
}

// Summarize collects the current ledger and lifetime counters.
func (s *Service) Summarize() (Summary, error) {
	state, err := s.Store.Read()
	if err != nil {
		return Summary{}, err
	}
	sum := Summary{Tracked: len(state.Items), Graves: len(state.Burials), Lifetime: state.Stats}
	for _, item := range state.Items {
		if sum.NextDue == nil || item.ExpiresAt.Before(*sum.NextDue) {
			t := item.ExpiresAt
			sum.NextDue = &t
		}
	}
	for _, b := range state.Burials {
		sum.GraveBytes += b.Size
		if sum.NextPurge == nil || b.PurgeAt.Before(*sum.NextPurge) {
			t := b.PurgeAt
			sum.NextPurge = &t
		}
	}
	return sum, nil
}
