//go:build windows

package doctor

import (
	"errors"
	"strings"
	"testing"
)

func TestDoctorRegistryTransactions(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		calls := 0
		got := diagnoseRegistryTransactions(func() error {
			calls++
			if unavailable {
				return errors.New("KTM disabled")
			}
			return nil
		})
		if calls != 1 || (got.Status == "WARN") != unavailable || (got.Status == "PASS") == unavailable || got.KTM.Available == unavailable {
			t.Fatalf("KTM probe count/status: calls=%d %+v", calls, got)
		}
		if unavailable {
			for _, detail := range []string{"KTM disabled", "context-menu", "login-startup", "fail closed"} {
				if !strings.Contains(got.Detail, detail) {
					t.Fatalf("missing KTM impact %q: %+v", detail, got)
				}
			}
		}
	}
}
