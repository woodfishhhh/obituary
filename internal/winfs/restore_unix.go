//go:build linux || darwin

package winfs

import (
	"fmt"
	"os"
	"path/filepath"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/unix"
)

type restoreParent struct {
	file      *os.File
	path      string
	id        corefs.Identity
	created   bool
	checkPath func() error
}

func (p *restoreParent) BeforeRestoreRename(check func() error) { p.checkPath = check }

func (p *restoreParent) Close() error              { return p.file.Close() }
func (p *restoreParent) Path() string              { return p.path }
func (p *restoreParent) Identity() corefs.Identity { return p.id }
func (p *restoreParent) fd() int                   { return int(p.file.Fd()) }
func (p *restoreParent) Rollback() error {
	if p.created {
		// POSIX cannot unlink a directory by its creation fd. A pathname
		// recheck followed by unlinkat could remove somebody else's directory.
		return fmt.Errorf("new restore parent retained at %s; inspect and remove it manually if empty", p.path)
	}
	return nil
}

func (fs FS) OpenRestoreParent(path string) (corefs.RestoreParent, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	st, err := fs.statAt(fd, "")
	if err != nil {
		f.Close()
		return nil, err
	}
	id, err := identity(&st, path)
	if err != nil {
		f.Close()
		return nil, err
	}
	real, err := Canonical(path)
	if err != nil {
		f.Close()
		return nil, err
	}
	back, err := fs.Identify(real)
	if err != nil || back != id {
		f.Close()
		return nil, &corefs.RestoreParentError{Kind: "pinned", Path: path, Cause: corefs.ErrUnsafe}
	}
	return &restoreParent{file: f, path: real, id: id}, nil
}

func (fs FS) MkdirRestoreParent(parent corefs.RestoreParent, name string) (corefs.RestoreParent, error) {
	p, ok := parent.(*restoreParent)
	if !ok || !singleName(name) {
		return nil, fmt.Errorf("invalid restore parent or filename: %w", corefs.ErrUnsafe)
	}
	err := unix.Mkdirat(p.fd(), name, 0700)
	created := err == nil
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	fd, err := unix.Openat(p.fd(), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(p.path, name)
	f := os.NewFile(uintptr(fd), path)
	st, err := fs.statAt(fd, "")
	if err != nil {
		f.Close()
		return nil, err
	}
	id, err := identity(&st, path)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &restoreParent{file: f, path: path, id: id, created: created}, nil
}

// FindRestoredIn supplies a spelling, never an identity assertion. The pinned
// parent and each candidate must still match the durable restore intent.
func (fs FS) FindRestoredIn(path string, parent, source corefs.Identity) (string, error) {
	if err := parent.RequirePersistent(); err != nil {
		return "", err
	}
	real, err := fs.Resolve(path)
	if err != nil {
		return "", fmt.Errorf("cannot locate recorded restore parent: %w", err)
	}
	opened, err := fs.OpenRestoreParent(real)
	if err != nil {
		return "", err
	}
	defer opened.Close()
	p := opened.(*restoreParent)
	if p.id != parent {
		return "", fmt.Errorf("recorded restore parent is no longer at %s; identity lookup is unavailable; retain record", path)
	}
	return fs.findRestoredChild(p, source)
}

func (fs FS) FindRestored(parent, source corefs.Identity) (string, error) {
	return fs.FindRestoredIn(parent.Volume, parent, source)
}

func (fs FS) findRestoredChild(parent *restoreParent, source corefs.Identity) (string, error) {
	entries, err := parent.file.ReadDir(-1)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		fd, _, err := fs.pinMoveSource(parent, entry.Name(), source)
		if err != nil {
			continue
		}
		unix.Close(fd)
		if path := fs.verifiedChildPath(parent, entry.Name(), source); path != "" {
			return path, nil
		}
	}
	// A failed candidate inspection cannot prove absence of the inode.
	return "", fmt.Errorf("restored object could not be verified in its recorded parent; retain record")
}

func (fs FS) RestoreParentPath(path string) (string, error) {
	fd, err := unix.Open(path, restoreParentPathOpenFlags(), 0)
	if err != nil {
		return "", err
	}
	defer unix.Close(fd)
	return restoreParentPath(fd, path)
}

func (fs FS) VerifyRestoreParentChain(records []corefs.RestoreParentRecord) error {
	if len(records) == 0 {
		return nil
	}
	ordered := make([]corefs.RestoreParentRecord, len(records))
	for i, record := range records {
		ordered[len(records)-1-i] = record
	}
	root := filepath.VolumeName(ordered[0].Path) + string(filepath.Separator)
	fd, err := unix.Open(root, restoreParentChainOpenFlags(), 0)
	if err != nil {
		return restoreParentChainError(ordered[0].Path, err)
	}
	defer func() { unix.Close(fd) }()
	reparseAncestor := false
	for i, record := range ordered {
		if i == 0 {
			if err := fs.verifyRestoreParentHandle(fd, record); err != nil {
				return err
			}
			continue
		}
		if filepath.Dir(record.Path) != ordered[i-1].Path {
			return restoreParentChainError(record.Path, corefs.ErrUnsafe)
		}
		child, err := fs.verifyRestoreParentChild(fd, record, reparseAncestor)
		if err != nil {
			return err
		}
		unix.Close(fd)
		fd = child
		reparseAncestor = reparseAncestor || record.Reparse
	}
	return nil
}

func (fs FS) verifyRestoreParentChild(fd int, record corefs.RestoreParentRecord, reparseAncestor bool) (int, error) {
	name := filepath.Base(record.Path)
	st, err := fs.statAt(fd, name)
	if err != nil {
		return -1, restoreParentChainError(record.Path, err)
	}
	isLink := st.Mode&unix.S_IFMT == unix.S_IFLNK
	if isLink != record.Reparse {
		return -1, &corefs.RestoreParentError{Kind: "link", Path: record.Path, Cause: corefs.ErrUnsafe}
	}
	if record.Reparse {
		id, err := fs.IdentifyFollowing(record.Path)
		if err != nil {
			return -1, restoreParentChainError(record.Path, err)
		}
		if !restoreIdentityMatches(record.Identity, id) {
			return -1, restoreParentChainError(record.Path, corefs.ErrUnsafe)
		}
		child, err := unix.Openat(fd, name, restoreParentChainFollowFlags(), 0)
		if err != nil {
			return -1, restoreParentChainError(record.Path, err)
		}
		return child, nil
	}
	child, err := unix.Openat(fd, name, restoreParentChainOpenFlags(), 0)
	if err != nil {
		return -1, restoreParentChainError(record.Path, err)
	}
	childStat, err := fs.statAt(child, "")
	if err == nil && childStat.Mode&unix.S_IFMT != unix.S_IFDIR {
		err = corefs.ErrUnsafe
	}
	if err == nil {
		id, identityErr := identity(&childStat, record.Path)
		if identityErr != nil {
			err = identityErr
		} else if !restoreIdentityMatches(record.Identity, id) {
			err = corefs.ErrUnsafe
		} else if !reparseAncestor {
			err = restoreParentPathMatches(child, record.Path)
		}
	}
	if err != nil {
		unix.Close(child)
		return -1, restoreParentChainError(record.Path, err)
	}
	return child, nil
}

func (fs FS) verifyRestoreParentHandle(fd int, record corefs.RestoreParentRecord) error {
	st, err := fs.statAt(fd, "")
	if err != nil {
		return restoreParentChainError(record.Path, err)
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR || record.Reparse {
		return &corefs.RestoreParentError{Kind: "link", Path: record.Path, Cause: corefs.ErrUnsafe}
	}
	id, err := identity(&st, record.Path)
	if err != nil {
		return restoreParentChainError(record.Path, err)
	}
	if !restoreIdentityMatches(record.Identity, id) {
		return restoreParentChainError(record.Path, corefs.ErrUnsafe)
	}
	return nil
}

func restoreIdentityMatches(want, got corefs.Identity) bool {
	return want.Serial == got.Serial && want.RequirePersistent() == nil && got.RequirePersistent() == nil && want.FileID == got.FileID
}

func restoreParentChainError(path string, err error) error {
	return &corefs.RestoreParentError{Kind: "unverified", Path: path, Cause: err}
}
