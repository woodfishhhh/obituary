//go:build ignore

// Fake installed CLI for packaging tests; never touches OS integrations.
package main

import (
	"encoding/json"
	"os"
	"strings"
)

func main() {
	if paths := os.Getenv("OBIT_INSTALL_SWAP_PATHS"); paths != "" {
		results := make(map[string]bool)
		for _, path := range strings.Split(paths, "|") {
			err := os.Rename(path, path+".saved")
			results[path] = err == nil
			if err == nil {
				_ = os.WriteFile(path, []byte("USER REPLACEMENT"), 0600)
			}
		}
		data, _ := json.Marshal(results)
		_ = os.WriteFile(os.Getenv("OBIT_INSTALL_SWAP_LOG"), data, 0600)
	}
	data, err := json.Marshal(os.Args[1:])
	if err == nil {
		err = os.WriteFile(os.Getenv("OBIT_INSTALL_UNINSTALL_LOG"), data, 0o600)
	}
	if err != nil || os.Getenv("OBIT_INSTALL_UNINSTALL_FAIL") == "1" {
		os.Exit(1)
	}
}
