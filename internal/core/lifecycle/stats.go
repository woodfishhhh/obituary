package lifecycle

import (
	"os"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

func treeSize(path string) int64 {
	info, err := os.Lstat(path)
	if err != nil || isNonFollowedEntry(info) {
		return 0
	}
	if !info.IsDir() {
		if info.Mode().IsRegular() {
			return info.Size()
		}
		return 0
	}
	total, _ := measureDirSize(path, true)
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
	return s.Store.UpdateGraveyard(func(st *State) error {
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
	Tracked           int        `json:"tracked"`
	NextDue           *time.Time `json:"nextDue,omitempty"`
	Graves            int        `json:"graves"`
	GraveBytes        int64      `json:"graveBytes"`
	GraveMaxBytes     int64      `json:"graveMaxBytes,omitempty"`
	QuotaPendingCount int        `json:"quotaPendingCount,omitempty"`
	NextPurge         *time.Time `json:"nextPurge,omitempty"`
	Lifetime          Stats      `json:"lifetime"`
}

// Summarize collects the current ledger and lifetime counters.
func (s *Service) Summarize() (Summary, error) {
	state, graveyard, err := s.Store.ReadSummary()
	if err != nil {
		return Summary{}, err
	}
	sum := Summary{
		Tracked:           len(state.Items),
		Graves:            graveyard.Count,
		GraveBytes:        graveyard.Bytes,
		GraveMaxBytes:     s.GraveyardMaxBytes,
		QuotaPendingCount: ledger.QuotaPendingCount(state),
		Lifetime:          state.Stats,
	}
	for _, item := range state.Items {
		if sum.NextDue == nil || item.ExpiresAt.Before(*sum.NextDue) {
			t := item.ExpiresAt
			sum.NextDue = &t
		}
	}
	if graveyard.Count != 0 {
		t := graveyard.NextPurge
		sum.NextPurge = &t
	}
	return sum, nil
}
