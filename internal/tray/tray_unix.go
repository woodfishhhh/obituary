//go:build (darwin && !desktop) || (linux && !desktop)

package tray

import (
	"fmt"
	"log"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func Run(*lifecycle.Service, *log.Logger) error {
	return fmt.Errorf("the system tray is Windows-only: %w", corefs.ErrUnsupported)
}
