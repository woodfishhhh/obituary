package ledger

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// viewEntry is the hot ledger the resident decoded for its last presentation
// request, with the exact store.json bytes it came from.
type viewEntry struct {
	mu sync.Mutex
	viewData
}

type viewData struct {
	wire     []byte
	state    State
	snapshot string // the graveyard snapshot that was verified; empty when the ledger has none
	stamp    snapshotStamp
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
// presentation requests between two commits. It decodes store.json only when its
// bytes differ from the ones decoded last, and it verifies the immutable
// graveyard snapshot once per size and modification time instead of per request.
// Anything it cannot vouch for (a ledger that needs migration, a snapshot that
// does not verify) is read by ReadActive, so the answer is the one a separate
// process would give. The state is shared with later requests: callers only read it.
func (s Store) ReadActiveView() (State, error) {
	if s.resident == nil {
		return s.ReadActive()
	}
	raw, err := readShared(s.statePath())
	if err != nil {
		return s.ReadActive()
	}
	if st, ok := s.resident.view.hit(raw); ok {
		return st, nil
	}
	decoded, events, err := s.decodeHot(raw)
	if err != nil {
		return s.ReadActive()
	}
	entry := viewData{wire: raw}
	if decoded.Graveyard != nil {
		// Stamped before it is verified: a change in between only costs a re-read.
		var ok bool
		entry.snapshot = filepath.Join(s.Home, decoded.Graveyard.File)
		if entry.stamp, ok = stampOf(entry.snapshot); !ok {
			return s.ReadActive()
		}
	}
	st, _, ok := s.committedSummary(decoded, events, nil)
	if !ok {
		return s.ReadActive()
	}
	entry.state = st
	s.resident.view.put(entry)
	return st, nil
}

func (v *viewEntry) hit(raw []byte) (State, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.wire == nil || !bytes.Equal(raw, v.wire) {
		return State{}, false
	}
	if v.snapshot != "" {
		if stamp, ok := stampOf(v.snapshot); !ok || stamp != v.stamp {
			return State{}, false
		}
	}
	return v.state, true
}

func (v *viewEntry) put(entry viewData) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.viewData = entry
}
