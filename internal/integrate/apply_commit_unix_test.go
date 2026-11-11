//go:build linux || darwin

package integrate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestPOSIXInstallationCommitPreservesExistingAndAbsentFiles(t *testing.T) {
	for _, mode := range []string{"create", "update", "remove"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "CLAUDE.md")
			before := "user notes\n"
			if mode == "remove" {
				// Only a marked reference is Obituary's to remove.
				before = referenceBegin + "\n@OBIT.md\n" + referenceEnd + "\n"
			}
			if mode != "create" {
				if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			change, err := planReference(Scope{Project: dir}, path, "@OBIT.md", mode == "remove")
			if err != nil {
				t.Fatal(err)
			}
			if err := Apply([]Change{change}); !errors.Is(err, core.ErrUnsupported) {
				t.Fatalf("commit did not fail closed: %v", err)
			}
			data, err := os.ReadFile(path)
			if mode == "create" {
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("created unsupported installation: %q %v", data, err)
				}
			} else if err != nil || string(data) != before {
				t.Fatalf("changed user file: %q %v", data, err)
			}
			entries, err := os.ReadDir(dir)
			want := 1
			if mode == "create" {
				want = 0
			}
			if err != nil || len(entries) != want {
				t.Fatalf("left commit artifacts: %v %v", entries, err)
			}
		})
	}
}
