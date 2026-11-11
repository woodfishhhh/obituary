//go:build linux || darwin

package integrate

import (
	"fmt"

	"github.com/woodfishhhh/obituary/internal/core"
)

// This registration path promises containment of all CLI descendants. Keep it
// disabled until POSIX process containment has an approved implementation.
// Direct MCP config-file installation and hook installation remain available.
func runClaude(string, []string) ([]byte, error) {
	return nil, fmt.Errorf("the Claude CLI MCP registration path is unavailable here; use config-file registration: %w", core.ErrUnsupported)
}
