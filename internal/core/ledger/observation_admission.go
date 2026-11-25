package ledger

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// AcceptObservations is the resident's durable receipt for hook requests. All
// rows reach disk with one write, in the observations.jsonl that every release
// already imports, so a replay finds them even if this process dies right after
// returning. Nothing here reads the ledger or runs Git; the caller acknowledges
// the hook only after this returns.
//
// While the consumer holds observations.lock, or once the file would pass its
// size limit, the rows go to the independent per-request files instead; both
// forms are replayed by token, so a request that arrives twice is registered once.
func (s Store) AcceptObservations(rows []Observation) error {
	var batch bytes.Buffer
	enc := json.NewEncoder(&batch)
	for _, row := range rows {
		if row.Identity == nil {
			return errors.New("observation requires a verified identity")
		}
		if err := validateObservation(row); err != nil {
			return err
		}
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	if appended, err := s.appendObservations(batch.Bytes()); err != nil || appended {
		return err
	}
	return s.DeferObservations(rows)
}

func (s Store) appendObservations(batch []byte) (bool, error) {
	if len(batch) > observationBytes {
		return false, nil
	}
	release, ok, err := TryLockExclusive(filepath.Join(s.Home, "observations.lock"))
	if err != nil || !ok {
		return false, err
	}
	defer release()
	path := filepath.Join(s.Home, "observations.jsonl")
	raw, err := readShared(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if len(raw)+len(batch)+1 > observationBytes {
		return false, nil
	}
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		raw = append(raw, '\n')
	}
	return true, writeDurably(path, append(raw, batch...))
}
