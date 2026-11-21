package doctor

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

func (lang diagnostics) diagnoseSlowGit(home string, now time.Time) DoctorCheck {
	c := DoctorCheck{Name: "slow-git", Status: "PASS"}
	var latest ledger.Event
	seen := make(map[ledger.Event]bool)
	for _, name := range []string{"audit.jsonl.1", "audit.jsonl"} {
		if err := scanGitRetries(filepath.Join(home, name), now, &c, &latest, seen); err != nil {
			c.Status, c.Detail = "FAIL", err.Error()
			return c
		}
	}
	c.Detail = fmt.Sprintf(lang.Text("cli.doctor.git_verification_timeouts_in_the_last_hours", nil), c.Errors)
	if c.Errors >= 3 {
		c.Status = "WARN"
		c.Detail += fmt.Sprintf(lang.Text("cli.doctor.automatic_cleanup_kept_files_in_place_and", nil), latest.ID, latest.Detail)
	}
	return c
}

func scanGitRetries(path string, now time.Time, c *DoctorCheck, latest *ledger.Event, seen map[ledger.Event]bool) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64<<10), 1<<20) // same line bound as core.Store.History
	for scanner.Scan() {
		var event ledger.Event
		if json.Unmarshal(scanner.Bytes(), &event) != nil || !gitTimeoutRetry(event, now) {
			continue
		}
		event.Time = event.Time.UTC()
		if seen[event] {
			continue
		}
		seen[event] = true
		c.Errors++
		if !event.Time.Before(latest.Time) {
			*latest = event
		}
	}
	return scanner.Err()
}

func gitTimeoutRetry(event ledger.Event, now time.Time) bool {
	return event.Action == ledger.ActionRetry && !event.Time.Before(now.Add(-24*time.Hour)) && !event.Time.After(now) &&
		strings.Contains(event.Detail, "context deadline exceeded") &&
		(strings.HasPrefix(event.Detail, "could not ask Git whether it is tracked:") ||
			strings.HasPrefix(event.Detail, "final Git verification budget exhausted:"))
}
