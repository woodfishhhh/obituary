//go:build ignore

// Real uninstall command with OS boundaries supplied by a compile-time overlay.
// This cannot compile without the test-only constructor, preventing accidental
// execution of the production registry/scheduler boundary in packaging tests.
package main

import (
	"fmt"
	"os"

	"github.com/woodfishhhh/obituary/internal/cli/uninstall"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/integrate"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if err := os.Chdir(os.Getenv("OBIT_USER_HOME")); err != nil {
		return err
	}
	if len(os.Args) > 1 && os.Args[1] == "seed" {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		changes, err := integrate.PlanClaudeExec(integrate.Scope{Global: true}, exe, false)
		if err != nil {
			return err
		}
		if err := integrate.Apply(changes); err != nil {
			return err
		}
		return (core.Store{Home: os.Getenv("OBIT_HOME")}).Update(func(*core.State) error { return nil })
	}
	cmd := uninstall.NewInstallerTestCmd()
	cmd.SetArgs(os.Args[2:])
	return cmd.Execute()
}
