//go:build linux || darwin

package integrate

import (
	"errors"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestPOSIXClaudeCLIRegistrationUnavailable(t *testing.T) {
	out, err := runClaude("must-not-execute", []string{"mcp", "add-json", "obituary", "{}"})
	if !errors.Is(err, core.ErrUnsupported) || len(out) != 0 {
		t.Fatalf("unsupported process containment must refuse before executing a client: %q %v", out, err)
	}
}
