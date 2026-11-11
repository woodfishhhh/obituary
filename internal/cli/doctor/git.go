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

	"github.com/woodfishhhh/obituary/internal/core"
)

func diagnoseSlowGit(home string, now time.Time) DoctorCheck {
	c := DoctorCheck{Name: "slow-git", Status: "PASS"}
	var latest core.Event
	seen := make(map[core.Event]bool)
	for _, name := range []string{"audit.jsonl.1", "audit.jsonl"} {
		if err := scanGitRetries(filepath.Join(home, name), now, &c, &latest, seen); err != nil {
			c.Status, c.Detail = "FAIL", err.Error()
			return c
		}
	}
	c.Detail = fmt.Sprintf("%d Git verification timeouts in the last 24 hours (available audit logs)", c.Errors)
	if c.Errors >= 3 {
		c.Status = "WARN"
		c.Detail += fmt.Sprintf("; automatic cleanup kept files in place and will retry; slow Git startup/query or index verification exceeded its budget; latest #%d: %s; inspect obit list and Git performance in that project", latest.ID, latest.Detail)
	}
	return c
}

func scanGitRetries(path string, now time.Time, c *DoctorCheck, latest *core.Event, seen map[core.Event]bool) error {
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
		var event core.Event
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

func gitTimeoutRetry(event core.Event, now time.Time) bool {
	return event.Action == core.ActionRetry && !event.Time.Before(now.Add(-24*time.Hour)) && !event.Time.After(now) &&
		strings.Contains(event.Detail, "context deadline exceeded") &&
		(strings.HasPrefix(event.Detail, "could not ask Git whether it is tracked:") ||
			strings.HasPrefix(event.Detail, "final Git verification budget exhausted:"))
}
