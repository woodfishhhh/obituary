package fileio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// writeDurably writes to a temporary file, flushes it to disk and then
// replaces path, so a crash leaves either the old or the new content.
func writeDurably(path string, data []byte) error {
	// Go's os.Root.Rename uses handle-relative Renameat on Windows, including
	// POSIX replacement and its fallback. Both names use the same parent.
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("write store: %w", err)
	}
	defer root.Close()
	name := filepath.Base(path)
	tmp := name + ".tmp"
	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("write store: %w", err)
	}
	if _, err := writeStoreBytes(f, data); err != nil {
		f.Close()
		return fmt.Errorf("write store: %w", err)
	}
	if err := syncFile(f); err != nil {
		f.Close()
		return fmt.Errorf("flush store: %w", err)
	}
	return replaceDurably(root, tmp, name, f)
}

// SyncReplacement is replaceDurably's post-rename failure injection seam.
var SyncReplacement = func(file *os.File) error { return syncFile(file) }

var WriteStoreBytes = (*os.File).Write

var RenameStore = func(root *os.Root, tmp, name string) error { return root.Rename(tmp, name) }

type unconfirmedCommit struct{ cause error }

func (e *unconfirmedCommit) Error() string {
	return "flush replaced file (commit unconfirmed): " + e.cause.Error()
}

func (e *unconfirmedCommit) Unwrap() error { return e.cause }

// replaceDurably owns f. Go 1.26 internal/syscall/windows/at_windows.go
// Renameat does not write through rename metadata. Microsoft File Caching
// (learn.microsoft.com/windows/win32/fileio/file-caching) requires
// FlushFileBuffers for metadata; Sync the same write handle after Root.Rename.
// A failed flush means commit is unconfirmed, so callers retain recovery data.
func replaceDurably(root *os.Root, tmp, name string, f *os.File) (err error) {
	defer func() {
		err = errors.Join(err, f.Close())
	}()
	if err := renameStore(root, tmp, name); err != nil {
		return fmt.Errorf("replace store: %w", err)
	}
	if err := syncReplacement(f); err != nil {
		return &unconfirmedCommit{cause: err}
	}
	if err := syncParentDirectory(root); err != nil {
		return &unconfirmedCommit{cause: err}
	}
	return nil
}

func WriteDurably(path string, data []byte) error { return writeDurably(path, data) }

func ReplaceDurably(root *os.Root, tmp, name string, file *os.File) error {
	return replaceDurably(root, tmp, name, file)
}

func IsCommitUnconfirmed(err error) bool {
	var unconfirmed *unconfirmedCommit
	return errors.As(err, &unconfirmed)
}

func syncReplacement(file *os.File) error { return SyncReplacement(file) }

func writeStoreBytes(file *os.File, data []byte) (int, error) { return WriteStoreBytes(file, data) }

func renameStore(root *os.Root, tmp, name string) error { return RenameStore(root, tmp, name) }
