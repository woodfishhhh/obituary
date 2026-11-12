//go:build linux || darwin

package regtxn

import (
	"fmt"

	core "github.com/woodfishhhh/obituary/internal/core/fs"
)

func Probe() error {
	return fmt.Errorf("registry transactions are Windows-only: %w", core.ErrUnsupported)
}
