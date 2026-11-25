package winfs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/windows"
)

// OpenCommitFile holds each ancestor against rename and opens reparse points literally.
// DeleteCommitFile applies deletion to the very handle whose identity was checked.
func (FS) PurgeGrave(path string, want fs.Identity, protected fs.ProtectedIDs) error {
	return purgeEntry(path, want, true, protected)
}
func purgeEntry(path string, root fs.Identity, top bool, protected fs.ProtectedIDs) error {
	f, err := OpenCommitFile(path, windows.DELETE)
	if err != nil {
		return err
	}
	defer f.Close()
	id, mode, err := inspectHandle(windows.Handle(f.Fd()), root.Volume)
	if err != nil {
		return err
	}
	if id.Serial != root.Serial || top && (id != root || !mode.IsDir()) || protected[id.FileID] {
		return fmt.Errorf("changed or protected grave entry %s: %w", path, fs.ErrUnsafe)
	}
	if mode.IsDir() {
		if err := purgeChildren(f, path, root, protected); err != nil {
			return err
		}
	}

	return DeleteCommitFile(f)
}

func purgeChildren(f *os.File, path string, root fs.Identity, protected fs.ProtectedIDs) error {
	var attrs windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &attrs); err != nil {
		return err
	}
	if attrs.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fs.ErrUnsafe
	}
	for {
		names, err := f.Readdirnames(64)
		for _, name := range names {
			if strings.ContainsRune(name, utf8.RuneError) {
				return fs.ErrUnsafe
			}
			if err := purgeEntry(filepath.Join(path, name), root, false, protected); err != nil {
				return err
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}
