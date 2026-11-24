package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

// ReportLifespan is how long a report stays before it is buried; the
// graveyard keeps it for its usual retention after that.
const ReportLifespan = 7 * 24 * time.Hour

// ReportsDir is the one folder inside the protected data directory whose
// files may be given a lifespan (the guards exempt it), so reports written
// for people expire and are buried like any other tracked file.
func ReportsDir(home string) string { return filepath.Join(home, "reports") }

// ExpireReport gives a report in ReportsDir its lifespan, as obit mark would.
func ExpireReport(svc *core.Service, path string, now time.Time) error {
	results, err := svc.Add([]string{path}, core.AddOptions{ExpiresAt: now.Add(ReportLifespan), Source: "doctor", Note: "obit doctor report"})
	for _, r := range results {
		if r.Error != "" {
			err = errors.Join(err, fmt.Errorf("%s: %s", r.Ref, r.Error))
		}
	}
	return err
}
