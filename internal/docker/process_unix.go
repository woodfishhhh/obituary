//go:build !windows

package docker

import "os/exec"

func hideWindow(cmd *exec.Cmd) {}
