package guardrm

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
	"unicode/utf8"
)

// AuditEvent is one append-only receipt, independent of the ledger and its locks.
type AuditEvent struct {
	At         time.Time `json:"at"`
	Kind       string    `json:"kind"`
	Command    string    `json:"command,omitempty"`
	Equivalent string    `json:"equivalent,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	Count      int64     `json:"count,omitempty"`
}

func Record(home string, event AuditEvent) error {
	event.At = time.Now().UTC()
	event.Command = auditCommand(event.Command)
	event.Equivalent = auditCommand(event.Equivalent)
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(home, "delete-guard.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	// One append write per event: simultaneous hooks never read/replace a total.
	_, err = f.Write(append(data, '\n'))
	return errors.Join(err, f.Close())
}

// Bound both command fields before JSON escaping can multiply their size.
func auditCommand(command string) string {
	const limit = 8 << 10
	const suffix = " [truncated]"
	if len(command) <= limit {
		return command
	}
	end := limit - len(suffix)
	for end > 0 && !utf8.RuneStart(command[end]) {
		end--
	}
	return command[:end] + suffix
}

type Counts struct {
	Denied    int64 `json:"denied"`
	Buried    int64 `json:"buried"`
	Oversized int64 `json:"oversized"` // old oversized lines skipped during reading
}

func ReadCounts(home string) (Counts, error) {
	var counts Counts
	f, err := os.Open(filepath.Join(home, "delete-guard.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return counts, nil
	}
	if err != nil {
		return counts, err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 64<<10)
	var line []byte
	oversized := false
	for {
		part, err := r.ReadSlice('\n')
		if err != nil && !errors.Is(err, bufio.ErrBufferFull) && !errors.Is(err, io.EOF) {
			return counts, err
		}
		if !oversized {
			if len(line)+len(part) > 8<<20 {
				oversized = true
			} else {
				line = append(line, part...)
			}
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue // drain even oversized lines without retaining more bytes
		}
		if oversized {
			counts.Oversized++
		} else if len(line) > 0 {
			var ev AuditEvent
			if err := json.Unmarshal(line, &ev); err != nil {
				return counts, fmt.Errorf("delete guard audit: %w", err)
			}
			switch ev.Kind {
			case "denied":
				counts.Denied++
			case "buried":
				counts.Buried += ev.Count
			}
		}
		if errors.Is(err, io.EOF) {
			return counts, nil
		}
		line = line[:0]
		oversized = false
	}
}
