//go:build linux || darwin

package regtxn

import (
	"fmt"

	"github.com/woodfishhhh/obituary/internal/core"
)

func Probe() error {
	return fmt.Errorf("registry transactions are Windows-only: %w", core.ErrUnsupported)
}
