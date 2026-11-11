//go:build linux || darwin

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "obit-gui is not supported on this platform; use the obit CLI")
	os.Exit(1)
}
