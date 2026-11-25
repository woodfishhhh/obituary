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
func (FS) PurgeGrave(path string, want fs.Identity, plan fs.PurgePlan) error {
	return purgeEntry(path, want, &want, plan)
}
func purgeEntry(path string, root fs.Identity, expected *fs.Identity, plan fs.PurgePlan) error {
	f, err := OpenCommitFile(path, windows.DELETE)
	if err != nil {
		return err
	}
	defer f.Close()
	id, mode, err := inspectHandle(windows.Handle(f.Fd()), root.Volume)
	if err != nil {
		return err
	}
	top := expected != nil && *expected == root
	if id.Serial != root.Serial || expected != nil && id != *expected || top && !mode.IsDir() {
		return fmt.Errorf("changed or protected grave entry %s: %w", path, fs.ErrUnsafe)
	}
	if err := plan.WithObject(id.FileID, false, nil); err != nil {
		return err
	}
	if top {
		err = purgeEntry(filepath.Join(path, plan.Name), root, &plan.Object, plan)
	} else if mode.IsDir() {
		err = purgeChildren(f, path, root, plan)
	}
	if err != nil {
		return err
	}

	return plan.WithObject(id.FileID, false, func() error { return DeleteCommitFile(f) })
}

func purgeChildren(f *os.File, path string, root fs.Identity, plan fs.PurgePlan) error {
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
			if err := purgeEntry(filepath.Join(path, name), root, nil, plan); err != nil {
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
