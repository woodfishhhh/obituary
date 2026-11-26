package winfs

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"golang.org/x/sys/unix"
)

// Trash clients can empty files/ at any time. Stage outside files/ until the
// D-0043 identity check succeeds; a failed rollback stays out of their reach.
func (fs FS) Recycle(path string, want corefs.Identity) error {
	path, err := Canonical(path)
	if err != nil {
		return err
	}
	root, original, err := fs.trashRoot(path, want)
	if err != nil {
		return err
	}
	defer root.Close()
	if path == root.path || strings.HasPrefix(path, root.path+"/") || strings.HasPrefix(root.path, path+"/") {
		return fmt.Errorf("source overlaps system Trash: %w", corefs.ErrUnsafe)
	}
	files, err := fs.privateTrashDir(root, "files")
	if err != nil {
		return err
	}
	defer files.Close()
	info, err := fs.privateTrashDir(root, "info")
	if err != nil {
		return err
	}
	defer info.Close()
	stage, err := fs.privateTrashDir(root, ".obituary-pending")
	if err != nil {
		return err
	}
	defer stage.Close()
	source, err := fs.OpenRestoreParent(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer source.Close()
	// The desktop uses Path, not the storage name, when putting a file back.
	name := "obit-" + rand.Text()
	meta := name + ".trashinfo"
	fd, err := unix.Openat(info.fd(), meta, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), meta)
	_, err = fmt.Fprintf(f, "[Trash Info]\nPath=%s\nDeletionDate=%s\n", (&url.URL{Path: original}).EscapedPath(), time.Now().Format("2006-01-02T15:04:05"))
	if err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err == nil {
		err = unix.Fsync(info.fd())
	}
	if err != nil {
		return err
	}
	if _, err = fs.moveFrom(source.(*restoreParent), filepath.Base(path), want, stage, name, false); err != nil {
		return err
	}
	if _, err = fs.moveFrom(stage, name, want, files, name, true); err != nil {
		var mismatch *corefs.MoveMismatchError
		if errors.As(err, &mismatch) && mismatch.Retained != "" {
			return err
		}
		_, rb := fs.moveFrom(stage, name, want, source.(*restoreParent), filepath.Base(path), false)
		if rb != nil {
			return &corefs.MoveMismatchError{Source: path, Retained: filepath.Join(stage.path, name), Cause: err, Rollback: rb}
		}
		return err
	}
	return nil
}

func (fs FS) privateTrashDir(p *restoreParent, name string) (*restoreParent, error) {
	// The trashinfo commit needs a directory descriptor that supports fsync.
	child, err := fs.mkdirRestoreParent(p, name, unix.O_RDONLY)
	if err != nil {
		return nil, err
	}
	d := child.(*restoreParent)
	var st unix.Stat_t
	if err = unix.Fstat(d.fd(), &st); err == nil && (st.Uid != uint32(os.Geteuid()) || st.Mode&0077 != 0 || d.id.Serial != p.id.Serial) {
		err = fmt.Errorf("insecure Trash directory %s: %w", d.path, corefs.ErrUnsafe)
	}
	if err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

func (fs FS) trashRoot(path string, want corefs.Identity) (*restoreParent, string, error) {
	base, err := userdirs.TrashData()
	if err != nil {
		return nil, "", err
	}
	if base == path || strings.HasPrefix(base, path+"/") {
		return nil, "", corefs.ErrUnsafe
	}
	if err = os.MkdirAll(base, 0700); err != nil {
		return nil, "", err
	}
	base, err = filepath.EvalSymlinks(base)
	if err != nil {
		return nil, "", err
	}
	home, err := fs.OpenRestoreParent(base)
	if err != nil {
		return nil, "", err
	}
	defer home.Close()
	// st_dev does not distinguish bind mounts, but rename cannot cross them.
	parentPath := filepath.Dir(path)
	srcMount, err := trashMountID(parentPath)
	if err != nil {
		return nil, "", err
	}
	homeMount, err := trashMountID(base)
	if err != nil {
		return nil, "", err
	}
	if srcMount == homeMount {
		root, err := fs.privateTrashDir(home.(*restoreParent), "Trash")
		return root, path, err
	}
	for parentPath != "/" {
		up := filepath.Dir(parentPath)
		id, err := trashMountID(up)
		if err != nil {
			return nil, "", err
		}
		if id != srcMount {
			break
		}
		parentPath = up
	}
	top, err := fs.OpenRestoreParent(parentPath)
	if err != nil {
		return nil, "", err
	}
	defer top.Close()
	rel, err := filepath.Rel(top.Path(), path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") || top.Identity().Serial != want.Serial {
		return nil, "", corefs.ErrUnsafe
	}
	uid := strconv.Itoa(os.Geteuid())
	shared, err := fs.OpenRestoreParent(filepath.Join(top.Path(), ".Trash"))
	if err == nil {
		defer shared.Close()
		var st unix.Stat_t
		if err = unix.Fstat(shared.(*restoreParent).fd(), &st); err != nil {
			return nil, "", err
		}
		if st.Mode&unix.S_ISVTX != 0 && shared.Identity().Serial == want.Serial {
			root, err := fs.privateTrashDir(shared.(*restoreParent), uid)
			if err == nil {
				return root, rel, nil
			}
		}
	}
	root, err := fs.privateTrashDir(top.(*restoreParent), ".Trash-"+uid)
	return root, rel, err
}

func trashMountID(path string) (uint64, error) {
	var st unix.Statx_t
	if err := unix.Statx(unix.AT_FDCWD, path, 0, unix.STATX_MNT_ID, &st); err != nil {
		return 0, err
	}
	if st.Mask&unix.STATX_MNT_ID == 0 {
		return 0, fmt.Errorf("cannot identify Trash mount: %w", corefs.ErrUnsupported)
	}
	return st.Mnt_id, nil
}
