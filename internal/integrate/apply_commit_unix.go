//go:build linux || darwin

package integrate

import (
	"fmt"
	"os"

	"github.com/woodfishhhh/obituary/internal/core"
)

// D-0036's deny-write/delete handles and deletion by the verified handle have
// no POSIX equivalent. Preserve read-only plans; never fall back to reopening
// names for installation writes under the stronger merged commit contract.
func unsupportedCommit() error {
	return fmt.Errorf("installation commits require Windows handle guarantees; preview remains available: %w", core.ErrUnsupported)
}

func createIntegrationTemporary(*os.Root, string) (*os.File, error) { return nil, unsupportedCommit() }
func deleteIntegrationHandle(*os.File) error                        { return unsupportedCommit() }
func (*integrationTarget) removeOriginal(Change) error              { return unsupportedCommit() }
func (*integrationTarget) publish(Change, *os.File) error           { return unsupportedCommit() }
