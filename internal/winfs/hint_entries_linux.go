//go:build linux

package winfs

import "os"

type hintEntry struct {
	name       string
	directory  bool
	mountPoint bool
}

func readHintEntries(dir *os.File, limit int) ([]hintEntry, error) {
	names, err := dir.Readdirnames(limit)
	entries := make([]hintEntry, 0, len(names))
	for _, name := range names {
		entries = append(entries, hintEntry{name: name, directory: true})
	}
	return entries, err
}
