package update

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

func updateCommand(ctx context.Context, argv []string) (*exec.Cmd, error) {
	if argv[0] == "scoop" {
		// Scoop ships a PowerShell entry point alongside its cmd shim.
		// Invoke it directly, without asking cmd.exe to interpret argv.
		script, err := exec.LookPath("scoop.ps1")
		if err != nil {
			return nil, fmt.Errorf("find Scoop PowerShell shim: %w", err)
		}
		argv = append([]string{"powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script}, argv[1:]...)
	}
	if argv[0] == "powershell.exe" {
		system, err := windows.GetSystemDirectory()
		if err != nil {
			return nil, err
		}
		argv[0] = filepath.Join(system, "WindowsPowerShell", "v1.0", "powershell.exe")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	return cmd, nil
}
