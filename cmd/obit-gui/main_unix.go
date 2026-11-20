//go:build darwin || (linux && !desktop)

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "this build has no desktop window; use the obit CLI")
	os.Exit(1)
}
