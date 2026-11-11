package ledger

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unicode/utf8"
)

// Audit actions.
const (
	ActionAdded    = "added"
	ActionUpdated  = "updated"
	ActionExtended = "extended"
	ActionCanceled = "canceled"
	ActionRecycled = "recycled" // moved to the Recycle Bin (detail notes a new path if it had moved)
	ActionVanished = "vanished" // gone before it expired; nothing to do
	ActionReplaced = "replaced" // a different object now sits at the path; left alone
	ActionRefused  = "refused"  // failed a safety check at expiry; left alone
	ActionRetry    = "retry"    // recycling failed or could not be verified; will try again
	ActionSkipped  = "skipped"  // a newer lifespan or cancellation superseded this claim
)

// Event is one line of the audit log.
type Event struct {
	Time   time.Time `json:"time"`
	Action string    `json:"action"`
	ID     int64     `json:"id"`
	Path   string    `json:"path"`
	Detail string    `json:"detail,omitempty"`
}

// AuditError means the change itself was committed but its audit entry could
// not be written. Callers should warn rather than report the change as failed.
type AuditError struct{ Err error }

func (e *AuditError) Error() string { return fmt.Sprintf("could not write the audit log: %v", e.Err) }

func (e *AuditError) Unwrap() error { return e.Err }

// OnlyAuditErrors reports whether every leaf represents an already committed
// operation's audit failure. A joined ledger failure must still fail the call.
func OnlyAuditErrors(err error) bool {
	// Inspect each node directly: errors.As would accept a mixed join as soon
	// as it found one AuditError, hiding an uncommitted ledger failure.
	switch e := err.(type) { //nolint:errorlint // This classifies every branch, not whether any wrapped error matches.
	case *AuditError:
		return true
	case interface{ Unwrap() []error }:
		parts := e.Unwrap()
		for _, part := range parts {
			if !OnlyAuditErrors(part) {
				return false
			}
		}
		return len(parts) > 0
	case interface{ Unwrap() error }:
		return OnlyAuditErrors(e.Unwrap())
	}
	return false
}

const auditLimit = 4 << 20 // rotate after 4 MiB

const auditLineLimit = 1 << 20

// Bound the encoded line, including JSON escaping, to History's reader limit.
// Preserve operation identity; only diagnostic detail may be abbreviated.
func auditLine(e Event) ([]byte, error) {
	detail := e.Detail
	for {
		data, err := json.Marshal(e)
		if err != nil {
			return nil, err
		}
		if len(data)+1 < auditLineLimit {
			return append(data, '\n'), nil
		}
		if detail == "" {
			return nil, errors.New("audit event exceeds 1 MiB without detail")
		}
		n := len(detail) / 2
		for n > 0 && !utf8.RuneStart(detail[n]) {
			n--
		}
		detail = detail[:n]
		e.Detail = detail + " [truncated]"
	}
}

func (s Store) auditPath() string { return filepath.Join(s.Home, "audit.jsonl") }

// RecordCommitted records a durable change even if its caller cancels while
// waiting for the audit lock. WithContext preserves the hook's lockDeadline.
func (s Store) RecordCommitted(events ...Event) error {
	return s.WithContext(context.WithoutCancel(s.Context())).Record(events...)
}

// Record appends events to the audit log, rotating it when it grows too big.
// Rotation and appending share one lock across processes. Failures are
// returned as *AuditError.
func (s Store) Record(events ...Event) error {
	if len(events) == 0 {
		return nil
	}
	err := s.withLock("audit.lock", true, func() error {
		if info, err := os.Stat(s.auditPath()); err == nil && info.Size() > auditLimit {
			if err := replaceFile(s.auditPath(), s.auditPath()+".1"); err != nil {
				return fmt.Errorf("rotate: %w", err)
			}
		}
		f, err := os.OpenFile(s.auditPath(), os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o600)
		if err != nil {
			return err
		}
		info, err := f.Stat()
		if err == nil && info.Size() > 0 {
			var tail [1]byte
			_, err = f.ReadAt(tail[:], info.Size()-1)
			if err == nil && tail[0] != '\n' {
				// Preserve a torn append as its own invalid line. History skips
				// it; the following committed event stays independently readable.
				_, err = f.Write([]byte{'\n'})
			}
		}
		if err != nil {
			f.Close()
			return err
		}
		for _, e := range events {
			data, err := auditLine(e)
			if err == nil {
				_, err = f.Write(data)
			}
			if err != nil {
				f.Close()
				return err
			}
		}
		if err := syncFile(f); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	})
	if err != nil {
		return &AuditError{Err: err}
	}
	return nil
}

// History returns up to the last n audit events, oldest first.
func (s Store) History(n int) ([]Event, error) {
	f, err := os.Open(s.auditPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var events []Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64<<10), auditLineLimit)
	for scanner.Scan() {
		var e Event
		if json.Unmarshal(scanner.Bytes(), &e) == nil {
			events = append(events, e)
			if len(events) > n {
				events = events[1:]
			}
		}
	}
	return events, scanner.Err()
}
