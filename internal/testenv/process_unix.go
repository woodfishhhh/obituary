//go:build linux || darwin

package testenv

import "os/exec"

func quietChild(*exec.Cmd) {}
