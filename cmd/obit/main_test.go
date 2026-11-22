package main

import (
	"os"
	"testing"

	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestMain(m *testing.M) {
	os.Exit(testenv.Main(m))
}

func TestHookRootOnlyRegistersHookCommands(t *testing.T) {
	root := newRoot([]string{"hook", "claude"})
	commands := root.Commands()
	if len(commands) != 1 || commands[0].Name() != "hook" {
		t.Fatalf("hook root registered unrelated commands: %v", commands)
	}
}
