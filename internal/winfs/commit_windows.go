package winfs

import (
	"errors"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// OpenCommitFile denies default-stream writes and deletion while a caller
// verifies a replacement in a pinned parent. Named streams and security can
// still change: callers preserving them must compare their final snapshots.
// Extra access is for attributes
// or handle deletion. os.Open shares writes and cannot provide this contract.
// Shared with the installer (D-0036); publication closes this handle and calls
// RenameCommitFile immediately, retaining its documented close-to-rename window.
func OpenCommitFile(path string, access uint32) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(longPath(path))
	if err != nil {
		return nil, err
	}
	access |= windows.GENERIC_READ
	h, err := windows.CreateFile(name, access, windows.FILE_SHARE_READ, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open verified commit", Path: path, Err: err}
	}
	return os.NewFile(uintptr(h), path), nil
}

// DeleteCommitFile removes only the verified object, never a reopened name.
// Unsupported filesystems fail closed to preserve the checked handle identity
// through deletion.
func DeleteCommitFile(f *os.File) error {
	flags := uint32(windows.FILE_DISPOSITION_DELETE | windows.FILE_DISPOSITION_POSIX_SEMANTICS |
		windows.FILE_DISPOSITION_FORCE_IMAGE_SECTION_CHECK | windows.FILE_DISPOSITION_IGNORE_READONLY_ATTRIBUTE)
	return windows.SetFileInformationByHandle(windows.Handle(f.Fd()), windows.FileDispositionInfoEx,
		(*byte)(unsafe.Pointer(&flags)), uint32(unsafe.Sizeof(flags)))
}

// CreateCommitTemporaryAt exclusively creates a temporary file relative to the
// pinned parent. The caller supplies a single generated filename. Retain its
// deny-write/delete handle through publication or failure cleanup. Extra access
// lets Explorer copy security without requiring those rights in the installer.
func CreateCommitTemporaryAt(root *os.Root, name string, extraAccess ...uint32) (*os.File, error) {
	parent, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	n, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return nil, err
	}
	// Root.OpenFile cannot request DELETE or deny write/delete sharing.
	// Relative NtCreateFile preserves the pinned directory without resolving
	// root.Name() again.
	attrs := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(parent.Fd()), ObjectName: n, Attributes: windows.OBJ_CASE_INSENSITIVE}
	attrs.Length = uint32(unsafe.Sizeof(attrs))
	var h windows.Handle
	// FILE_GENERIC_* include SYNCHRONIZE. Backup intent preserves Explorer's
	// ability to copy named streams/extended attributes through this same handle.
	access := uint32(windows.FILE_GENERIC_READ | windows.FILE_GENERIC_WRITE | windows.DELETE)
	for _, extra := range extraAccess {
		access |= extra
	}
	err = windows.NtCreateFile(&h, access,
		&attrs, &windows.IO_STATUS_BLOCK{}, nil, windows.FILE_ATTRIBUTE_NORMAL,
		windows.FILE_SHARE_READ, windows.FILE_CREATE,
		windows.FILE_NON_DIRECTORY_FILE|windows.FILE_SYNCHRONOUS_IO_NONALERT|windows.FILE_OPEN_FOR_BACKUP_INTENT, 0, 0)
	path := filepath.Join(root.Name(), name) // diagnostic label, not an I/O path
	if err != nil {
		var status windows.NTStatus
		if errors.As(err, &status) {
			err = status.Errno()
		}
		return nil, &os.PathError{Op: "create commit temporary", Path: path, Err: err}
	}
	return os.NewFile(uintptr(h), path), nil
}

// commitRenameInfo mirrors FILE_RENAME_INFO; FileName extends past it.
type commitRenameInfo struct {
	Flags          uint32
	RootDirectory  windows.Handle
	FileNameLength uint32
	FileName       [1]uint16
}

// CommitRenameBuffer prepares the handle-relative rename before the caller
// releases the verified destination, keeping the move bound to that directory.
func CommitRenameBuffer(parent windows.Handle, target string, replace bool) ([]byte, error) {
	name, err := windows.UTF16FromString(target)
	if err != nil {
		return nil, err
	}
	size := int(unsafe.Offsetof(commitRenameInfo{}.FileName)) + len(name)*2
	buf := make([]byte, max(size, int(unsafe.Sizeof(commitRenameInfo{}))))
	info := (*commitRenameInfo)(unsafe.Pointer(&buf[0]))
	if replace {
		info.Flags = windows.FILE_RENAME_REPLACE_IF_EXISTS | windows.FILE_RENAME_POSIX_SEMANTICS
	}
	info.RootDirectory = parent
	info.FileNameLength = uint32((len(name) - 1) * 2)
	copy(unsafe.Slice(&info.FileName[0], len(name)), name)
	return buf, nil
}

// RenameCommitFile publishes exactly the open source relative to the pinned
// parent. SetFileInformationByHandle rejects this relative form on Windows;
// use NtSetInformationFile to keep the checked source handle throughout the
// move. Unsupported volumes fail closed.
func RenameCommitFile(file *os.File, buf []byte) error {
	const fileRenameInformationEx = 65 // FILE_INFORMATION_CLASS value from Windows ntifs.h
	err := windows.NtSetInformationFile(windows.Handle(file.Fd()), &windows.IO_STATUS_BLOCK{},
		&buf[0], uint32(len(buf)), fileRenameInformationEx)
	var status windows.NTStatus
	if errors.As(err, &status) {
		return status.Errno()
	}
	return err
}
