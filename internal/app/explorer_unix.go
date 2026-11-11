//go:build linux || darwin

package app

import (
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/core"
)

// Explorer labels have no POSIX projection. CLI diagnostics disclose this.
func syncExplorer(*core.Service, config.Explorer) bool { return true }
func syncCurrentExplorer(*core.Service)                {}
