package main

import (
	"github.com/woodfishhhh/obituary/internal/cli/hook"
	"os"
)

func main() {
	c := hook.NewCmd()
	c.SetArgs(os.Args[2:])
	if e := c.Execute(); e != nil {
		os.Exit(1)
	}
}
