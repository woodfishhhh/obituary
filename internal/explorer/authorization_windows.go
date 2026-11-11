package explorer

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"
	"unsafe"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/winfs"
	"golang.org/x/sys/windows"
)

func (p projection) add(svc *core.Service, path string, item core.Item, options Options, folders []string) error {
	dir := filepath.Dir(path)
	h, err := pinDirectory(dir)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	id, err := winfs.IdentifyHandle(h, dir)
	if err != nil {
		return err
	}
	auth, seen := p.auth[dir]
	if seen && auth.Identity != id {
		return errors.New("explorer directory changed during authorization")
	}
	if !seen {
		auth, err = authorizeDirectory(svc, dir, id, folders)
		if err != nil {
			return err
		}
	}
	name := filepath.Base(path)
	if err := verifyChild(h, dir, name, *item.Identity); err != nil {
		return err
	}
	p.auth[dir] = auth
	if !options.Everywhere && item.WatchID == 0 && item.Source != "watch" && !auth.InScope || auth.Project.Root != "" && !options.InGit {
		return nil
	}
	if p.dates[dir] == nil {
		p.dates[dir] = make(map[string]time.Time)
		p.dirs[directoryKey(id)] = directory{Path: dir, Identity: id, Candidate: true}
	}
	auth.Items[name] = *item.Identity
	p.dates[dir][name] = item.ExpiresAt
	return nil
}

func authorizeDirectory(svc *core.Service, dir string, id core.Identity, folders []string) (authorization, error) {
	inScope, err := inFolders(svc.FS, dir, folders)
	if err != nil {
		return authorization{}, err
	}
	proj, _, err := findProject(svc.Store.Context(), dir)
	return authorization{Identity: id, InScope: inScope, Project: proj, Items: make(map[string]core.Identity)}, err
}

func verifyChild(parent windows.Handle, dir, name string, want core.Identity) error {
	// Open relative to the pinned handle to preserve the checked parent.
	// os.Root.Open follows a final symlink; ledger IDs do not.
	n, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return err
	}
	attrs := windows.OBJECT_ATTRIBUTES{RootDirectory: parent, ObjectName: n, Attributes: windows.OBJ_CASE_INSENSITIVE}
	attrs.Length = uint32(unsafe.Sizeof(attrs))
	var h windows.Handle
	err = windows.NtCreateFile(&h, windows.FILE_READ_ATTRIBUTES, &attrs, &windows.IO_STATUS_BLOCK{}, nil, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, windows.FILE_OPEN,
		windows.FILE_OPEN_REPARSE_POINT|windows.FILE_OPEN_FOR_BACKUP_INTENT, 0, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	got, err := winfs.IdentifyHandle(h, filepath.Join(dir, name))
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("explorer child %q changed since locating the ledger item", name)
	}
	return nil
}

func (a authorization) verify(parent windows.Handle, dir string) error {
	for name, id := range a.Items {
		if err := verifyChild(parent, dir, name, id); err != nil {
			return err
		}
	}
	return nil
}
