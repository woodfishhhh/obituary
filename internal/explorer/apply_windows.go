package explorer

import (
	"bytes"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"github.com/woodfishhhh/obituary/internal/winfs"
	"golang.org/x/sys/windows"
)

var beforeLabelsCommit func() error
var beforeLabelsWrite func() error
var beforeLabelsPublish func(string) error
var beforeLabelsDelete func() error

func labelsCommitHook() error {
	if beforeLabelsCommit != nil {
		return beforeLabelsCommit()
	}
	return nil
}

// Apply rechecks the snapshot before touching either content or attributes.
// Temporary contents, metadata, publication and failure cleanup use the same
// protected file handle under the pinned directory.
func (p *Plan) Apply() error {
	handle, err := pinDirectory(p.dir)
	if err != nil {
		return err
	}
	defer func() {
		if handle != windows.InvalidHandle {
			windows.CloseHandle(handle)
		}
	}()
	if err := p.authorized.verify(handle, p.dir); err != nil {
		return err
	}
	root, err := os.OpenRoot(p.dir)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := p.check(root); err != nil {
		return err
	}
	if bytes.Equal(p.before.data, p.after) && !p.remove && p.before.attrs == p.fileAttrs && p.dirAttrs == p.folderAttrs {
		return nil
	}
	if p.remove && p.before.info == nil {
		return nil
	}
	if err := p.write(root); err != nil {
		return err
	}
	if err := setHandleAttributes(handle, p.dirAttrs|windows.FILE_ATTRIBUTE_READONLY); err != nil {
		return err
	}
	windows.CloseHandle(handle)
	handle = windows.InvalidHandle
	// Publish the change while folder customization is still enabled.
	// Root.Chmod restores only our readonly bit, through the same directory.
	notify(p.dir, p.names, p.iniEvent())
	mode := os.FileMode(0o700)
	if p.folderAttrs&windows.FILE_ATTRIBUTE_READONLY != 0 {
		mode = 0o500
	}
	return root.Chmod(".", mode)
}

func (p *Plan) iniEvent() uintptr {
	if p.remove {
		return 0x4
	} // SHCNE_DELETE: an absent path cannot be UPDATEITEM
	if p.before.info == nil {
		return 0x2
	} // SHCNE_CREATE
	return 0x2000 // SHCNE_UPDATEITEM
}

func (p *Plan) check(root *os.Root) error {
	info, err := root.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(info, p.directory) {
		return errors.New("explorer directory changed since planning")
	}
	attrs, err := attributes(p.dir)
	if err != nil {
		return err
	}
	current, err := readSnapshot(root)
	if err != nil {
		return err
	}
	same := current.info == nil && p.before.info == nil
	if current.info != nil && p.before.info != nil {
		same = os.SameFile(current.info, p.before.info)
	}
	if !same || !bytes.Equal(current.data, p.before.data) || current.attrs != p.before.attrs || attrs != p.dirAttrs {
		return errors.New("desktop.ini or folder attributes changed since planning")
	}
	return nil
}

func (p *Plan) write(root *os.Root) error {
	if beforeLabelsWrite != nil {
		if err := beforeLabelsWrite(); err != nil {
			return err
		}
	}
	old, err := p.commitOriginal()
	if err != nil {
		return err
	}
	if old != nil {
		defer old.Close()
		if err := p.prepareRemoval(old); err != nil {
			return err
		}
	}
	if p.remove {
		return p.deleteOriginal(old)
	}
	return p.writeReplacement(root, old)
}

func (p *Plan) prepareRemoval(old *os.File) error {
	if p.remove {
		if err := p.captureRemoval(old); err != nil {
			return err
		}
	}
	// Text ownership alone says nothing about other NTFS streams. When
	// the handle cannot prove whole-file ownership, only remove our text.
	if p.remove && !onlyDefaultStream(old) {
		p.remove = false
	}
	return nil
}

func (p *Plan) deleteOriginal(old *os.File) error {
	if beforeLabelsDelete != nil {
		if err := beforeLabelsDelete(); err != nil {
			return err
		}
	}
	if err := p.verifyFinal(old); err != nil {
		return err
	}
	return winfs.DeleteCommitFile(old)
}

func (p *Plan) writeReplacement(root *os.Root, old *os.File) (err error) {
	name := ".obit-labels-" + rand.Text() + ".tmp"
	f, err := winfs.CreateCommitTemporaryAt(root, name, windows.WRITE_DAC|windows.WRITE_OWNER)
	if err != nil {
		return err
	}
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, winfs.DeleteCommitFile(f))
		}
		err = errors.Join(err, f.Close())
	}()
	if err := p.copyOriginal(old, f); err != nil {
		return err
	}
	_, err = f.Write(p.after)
	if err == nil && !testing.Testing() {
		err = f.Sync()
	}
	if err != nil {
		return err
	}
	if beforeLabelsPublish != nil {
		if err := beforeLabelsPublish(filepath.Join(p.dir, name)); err != nil {
			return err
		}
	}
	if err := p.replace(root, f, old); err != nil {
		return err
	}
	published = true
	// Rename sets Archive on Windows. Restore attributes on the same handle
	// after replacement, so a concurrent editor's replacement is never changed.
	return setHandleAttributes(windows.Handle(f.Fd()), p.fileAttrs)
}

func (p *Plan) replace(root *os.Root, temporary, old *os.File) error {
	parent, err := root.Open(".")
	if err != nil {
		return err
	}
	defer parent.Close()
	buf, err := winfs.CommitRenameBuffer(windows.Handle(parent.Fd()), iniName, p.before.info != nil)
	if err != nil {
		return err
	}
	if old == nil {
		if err := p.check(root); err != nil {
			return err
		}
		if err := labelsCommitHook(); err != nil {
			return err
		}
		return winfs.RenameCommitFile(temporary, buf)
	}
	if err := p.verifyFinal(old); err != nil {
		return err
	}
	if p.before.attrs&windows.FILE_ATTRIBUTE_READONLY != 0 {
		// Keep a metadata-only handle after closing the exclusive read handle,
		// so a failed rename restores attributes on exactly the old object.
		h, err := openAttributes(filepath.Join(p.dir, iniName))
		if err != nil {
			return err
		}
		defer windows.CloseHandle(h)
		if err := setHandleAttributes(h, p.before.attrs&^windows.FILE_ATTRIBUTE_READONLY); err != nil {
			return err
		}
		defer setHandleAttributes(h, p.before.attrs)
	}
	// The protected temporary retains the old streams/ACL and the original
	// bytes in its receipt and all block-external edits. Only generated labels
	// are replaced, so no separate user-content backup is needed. D-0036's
	// accepted close-to-rename window remains; never remove the target first.
	if err := old.Close(); err != nil {
		return err
	}
	return winfs.RenameCommitFile(temporary, buf)
}

func openAttributes(path string) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES|windows.FILE_WRITE_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
}

// A directory handle without FILE_SHARE_DELETE keeps path-based attribute
// operations on our own temporary file inside the planned parent.
// FILE_LIST_DIRECTORY is required: metadata-only handles do not block rename.
func pinDirectory(path string) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	h, err := windows.CreateFile(p, windows.FILE_LIST_DIRECTORY|windows.FILE_READ_ATTRIBUTES|windows.FILE_WRITE_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return 0, err
	}
	var info windows.ByHandleFileInformation
	err = windows.GetFileInformationByHandle(h, &info)
	if err == nil && info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		err = errors.New("explorer directory is a reparse point")
	}
	if err != nil {
		windows.CloseHandle(h)
		return 0, err
	}
	return h, nil
}

func setHandleAttributes(h windows.Handle, attrs uint32) error {
	info := struct {
		Creation, Access, Write, Change int64
		Attributes                      uint32
		Padding                         uint32
	}{Attributes: attrs}
	return windows.SetFileInformationByHandle(h, windows.FileBasicInfo, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
}

func attributes(path string) (uint32, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	return windows.GetFileAttributes(p)
}

func setAttributes(path string, attrs uint32) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	if attrs == 0 {
		attrs = windows.FILE_ATTRIBUTE_NORMAL
	}
	return windows.SetFileAttributes(p, attrs)
}

var changeNotify = windows.NewLazySystemDLL("shell32.dll").NewProc("SHChangeNotify")

func notify(dir string, names []string, iniEvent uintptr) {
	for _, name := range names {
		p, _ := windows.UTF16PtrFromString(filepath.Join(dir, name))
		changeNotify.Call(0x2000, 0x3005, uintptr(unsafe.Pointer(p)), 0) // UPDATEITEM, PATHW | FLUSHNOWAIT
	}
	p, _ := windows.UTF16PtrFromString(filepath.Join(dir, iniName))
	if iniEvent == 0x2000 {
		// Root.Rename installs a new identity; invalidate the old INI entry
		// before publishing its replacement to already-open Explorer views.
		changeNotify.Call(0x4, 0x3005, uintptr(unsafe.Pointer(p)), 0)
		iniEvent = 0x2
	}
	changeNotify.Call(iniEvent, 0x3005, uintptr(unsafe.Pointer(p)), 0)
	p, _ = windows.UTF16PtrFromString(dir)
	changeNotify.Call(0x2000, 0x3005, uintptr(unsafe.Pointer(p)), 0)
	// Begin delivery before a short-lived CLI exits, without waiting for every
	// Shell component. FLUSH delays receipts and still leaves cached names in
	// already-open Explorer views; notification delivery is not a view refresh.
	changeNotify.Call(0x1000, 0x3005, uintptr(unsafe.Pointer(p)), 0) // UPDATEDIR, PATHW | FLUSHNOWAIT
}
