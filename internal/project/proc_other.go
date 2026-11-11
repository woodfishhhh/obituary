//go:build !windows

package project

import "os/exec"

const caseInsensitive = false

func hideWindow(*exec.Cmd) {}

func commandArgSize(arg string) int { return len(arg) + 1 }
