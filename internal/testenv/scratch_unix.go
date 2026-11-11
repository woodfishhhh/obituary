//go:build linux || darwin

package testenv

import (
	"io"
	"time"
)

// Abandoned-fixture recovery relies on Windows process creation times and
// pinned ACL restoration. POSIX tests only remove their own scratch in Main.
func reapScratch(string, time.Time, io.Writer) {}
func writeScratchOwner(string, string) error   { return nil }
func scratchPackage() string                   { return "" }
