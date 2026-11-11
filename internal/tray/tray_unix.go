//go:build linux || darwin

package tray

import (
	"fmt"
	"log"

	"github.com/woodfishhhh/obituary/internal/core"
)

func Run(*core.Service, *log.Logger) error {
	return fmt.Errorf("the system tray is Windows-only: %w", core.ErrUnsupported)
}
