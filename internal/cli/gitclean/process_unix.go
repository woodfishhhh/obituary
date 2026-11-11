//go:build linux || darwin

package gitclean

import (
	"os/exec"
)

func hideWindow(cmd *exec.Cmd) {}
