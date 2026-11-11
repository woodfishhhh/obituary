package ledger

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ObservationQueueStatus struct {
	Pending int       `json:"pending"`
	Dead    int       `json:"dead"`
	Partial int       `json:"partial"`
	Invalid int       `json:"invalid"`
	Oldest  time.Time `json:"oldest,omitzero"`
}

// InspectObservations is read-only, including for legacy and malformed input.
func (s Store) InspectObservations() (ObservationQueueStatus, error) {
	var status ObservationQueueStatus
	entries, err := os.ReadDir(s.observationDir())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return status, err
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			status.Partial++
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		status.Pending++
		row, err := readObservation(filepath.Join(s.observationDir(), entry.Name()))
		at := row.ObservedAt
		if err != nil || entry.Name() != observationName(row.Token) {
			status.Invalid++
			info, err := entry.Info()
			if err != nil {
				return status, err
			}
			at = info.ModTime()
		}
		if status.Oldest.IsZero() || at.Before(status.Oldest) {
			status.Oldest = at
		}
	}
	dead, err := os.ReadDir(filepath.Join(s.observationDir(), "dead"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return status, err
	}
	for _, entry := range dead {
		if strings.HasSuffix(entry.Name(), ".json") {
			status.Dead++
		}
	}
	err = s.inspectLegacyObservations(&status)
	return status, err
}

func (s Store) inspectLegacyObservations(status *ObservationQueueStatus) error {
	f, err := os.Open(filepath.Join(s.Home, "observations.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(f, observationBytes+1))
	if err != nil {
		return err
	}
	if len(data) > observationBytes {
		return errors.New("legacy observation queue exceeds 1 MiB")
	}
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		status.Pending++
		var row Observation
		err := json.Unmarshal(line, &row)
		at := row.ObservedAt
		if err != nil || validateObservation(row) != nil {
			status.Invalid++
			at = info.ModTime()
		}
		if status.Oldest.IsZero() || at.Before(status.Oldest) {
			status.Oldest = at
		}
	}
	return nil
}
