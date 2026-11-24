package ledger

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

// snapshotMemo is the verified projection of the graveyard snapshot the resident
// last read, with the reference that vouched for it and the file's size and time
// when it was read.
type snapshotMemo struct {
	mu    sync.Mutex
	ref   GraveyardReference
	stamp snapshotStamp
	snap  graveyardSummarySnapshot
	ok    bool
}

type snapshotStamp struct {
	size int64
	mod  time.Time
}

func stampOf(path string) (snapshotStamp, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return snapshotStamp{}, false
	}
	return snapshotStamp{info.Size(), info.ModTime()}, true
}

// ReadActiveView is ReadActive for the resident, which answers many
// presentation requests while hooks commit. Every call decodes the store.json it
// finds, so the answer is always the latest commit, but the immutable graveyard
// snapshot (most of the cold bytes ReadActive would read and hash again) is
// parsed once per file: the memo is used only while the hot ledger still
// references the same snapshot (same hash) and the file has the same size and
// modification time. The hot ledger is still checked against it on every call.
// Everything else is ReadActive, which a process that is not the resident uses.
func (s Store) ReadActiveView() (State, error) {
	if s.resident == nil {
		return s.ReadActive()
	}
	raw, err := readShared(s.statePath())
	if err != nil {
		return s.ReadActive()
	}
	decoded, events, err := s.decodeHot(raw)
	if err != nil {
		return s.ReadActive()
	}
	st, _, ok := s.committedSummary(decoded, events, nil, s.memoSummary)
	if !ok {
		return s.ReadActive()
	}
	return st, nil
}

func (s Store) memoSummary(st State) (GraveyardSummary, error) {
	memo := &s.resident.snapshot
	path := filepath.Join(s.Home, st.Graveyard.File)
	stamp, stamped := stampOf(path) // before the read: a change in between only costs a re-read
	snap, hit := memo.get(*st.Graveyard, stamp, stamped)
	if !hit {
		var err error
		if snap, err = s.loadSummarySnapshot(st); err != nil {
			return GraveyardSummary{}, err
		}
		if stamped {
			memo.put(*st.Graveyard, stamp, snap)
		}
	}
	return s.summarize(st, snap)
}

func (m *snapshotMemo) get(ref GraveyardReference, stamp snapshotStamp, stamped bool) (graveyardSummarySnapshot, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !stamped || !m.ok || m.ref != ref || m.stamp != stamp {
		return graveyardSummarySnapshot{}, false
	}
	return m.snap, true
}

func (m *snapshotMemo) put(ref GraveyardReference, stamp snapshotStamp, snap graveyardSummarySnapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ref, m.stamp, m.snap, m.ok = ref, stamp, snap, true
}
