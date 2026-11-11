package cmdutil

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/hook"
)

// Executable resolves the binary for one command or planning operation.
// The zero value uses the running executable.
type Executable func() (string, error)

func (e Executable) Path() (string, error) {
	if e == nil {
		return os.Executable()
	}
	return e()
}

func HookCommand(agent string) string {
	return Executable(nil).HookCommand(agent)
}

// HookCommand pins the executable so later PATH changes cannot redirect hooks.
func (e Executable) HookCommand(agent string) string {
	self, err := e.Path()
	if err == nil {
		if real, err := filepath.EvalSymlinks(self); err == nil {
			self = real
		}
		return fmt.Sprintf("%q hook %s", filepath.ToSlash(self), agent)
	}
	return ""
}

// wslHookCommand is the command Codex running inside WSL uses to reach this
// Windows binary through interop: /mnt/<drive>/... (empty if not applicable).
func WSLHookCommand(agent string) string {
	return Executable(nil).WSLHookCommand(agent)
}

func (e Executable) WSLHookCommand(agent string) string {
	self, err := e.Path()
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(self); err == nil {
		self = real
	}
	command := hook.WSLExecutable(self)
	if command == "" {
		return ""
	}
	return command + " hook " + agent
}
