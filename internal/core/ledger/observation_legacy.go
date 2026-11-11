package ledger

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Import the old handoff under its original lock. A crash may import a token
// twice; the request filename and ledger receipt both retain that same token.
func (s Store) migrateObservations() error {
	path := filepath.Join(s.Home, "observations.jsonl")
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(f, observationBytes+1))
	f.Close()
	if err != nil {
		return err
	}
	if len(data) > observationBytes {
		return errors.New("legacy observation queue exceeds 1 MiB")
	}
	if err := os.MkdirAll(s.observationDir(), 0o700); err != nil {
		return err
	}
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if err := s.importObservation(line); err != nil {
			return err
		}
	}
	return os.Remove(path)
}

func (s Store) importObservation(data []byte) error {
	var row Observation
	err := json.Unmarshal(data, &row)
	if err == nil {
		err = validateObservation(row)
	}
	if err == nil {
		path := filepath.Join(s.observationDir(), observationName(row.Token))
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return writeObservation(path, row)
	}
	dead := filepath.Join(s.observationDir(), "dead")
	if mkdirErr := os.MkdirAll(dead, 0o700); mkdirErr != nil {
		return mkdirErr
	}
	name := fmt.Sprintf("legacy-%x.json", sha256.Sum256(data))
	if writeErr := writeObservationFile(filepath.Join(dead, name), data); writeErr != nil {
		return writeErr
	}
	return s.Record(Event{Action: "observation dropped", Path: "observations.jsonl", Detail: err.Error()})
}
