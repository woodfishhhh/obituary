package testenv

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func processCreated() (uint64, error) {
	return processCreationTime(windows.CurrentProcess())
}

// A DACL-only handle works even for unreadable fixtures. It pins the name
// without following the leaf and without preventing a separate ACL-only open.
func openScratchEntry(path string) (windows.Handle, windows.Win32finddata, error) {
	return openFixtureHandle(path, windows.WRITE_DAC|windows.READ_CONTROL, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE)
}

func openFixtureHandle(path string, access, share uint32) (windows.Handle, windows.Win32finddata, error) {
	var info windows.Win32finddata
	parent, err := os.Open(filepath.Dir(path))
	if err != nil {
		return 0, info, err
	}
	defer parent.Close()
	n, err := windows.NewNTUnicodeString(filepath.Base(path))
	if err != nil {
		return 0, info, err
	}
	// Like winfs/longpath_windows.go, request exactly the required rights through
	// x/sys NtCreateFile. CreateFile also requests FILE_READ_ATTRIBUTES, which a
	// GENERIC_READ deny removes even when the owner can still restore the DACL.
	attrs := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(parent.Fd()), ObjectName: n, Attributes: windows.OBJ_CASE_INSENSITIVE}
	attrs.Length = uint32(unsafe.Sizeof(attrs))
	var h windows.Handle
	err = windows.NtCreateFile(&h, access, &attrs, &windows.IO_STATUS_BLOCK{}, nil, windows.FILE_ATTRIBUTE_NORMAL, share, windows.FILE_OPEN, windows.FILE_OPEN_REPARSE_POINT|windows.FILE_OPEN_FOR_BACKUP_INTENT, 0, 0)
	if err != nil {
		return 0, info, fmt.Errorf("open metadata: %w", err)
	}
	// Query the directory entry while this non-delete-sharing handle pins it.
	// Handle metadata queries require FILE_READ_ATTRIBUTES, denied by the very
	// fixtures being recovered; FindFirstFile reads the checked parent's entry.
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		windows.CloseHandle(h)
		return 0, info, err
	}
	search, err := windows.FindFirstFile(p, &info)
	if err != nil {
		windows.CloseHandle(h)
		return 0, info, fmt.Errorf("read metadata: %w", err)
	}
	windows.FindClose(search)
	return h, info, nil
}

func scratchCleanupACL(path string) (*windows.ACL, error) {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return nil, err
	}
	acl, _, err := sd.DACL()
	if err != nil {
		return nil, err
	}
	if acl == nil {
		return nil, errors.New("scratch has no DACL")
	}
	// Reuse the scratch's access grants (including restricted-token SIDs).
	// x/sys GetNamedSecurityInfo returns a private Go-heap copy. Clear inheritance
	// only on that copy so SetSecurityInfo cannot propagate ACEs into link targets.
	// Exact fixture DACLs died with the old process; this is cleanup access only.
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, i, &ace); err != nil {
			return nil, err
		}
		ace.Header.AceFlags = 0
	}
	return acl, nil
}

func processCreationTime(h windows.Handle) (uint64, error) {
	var created, exited, kernel, user windows.Filetime
	err := windows.GetProcessTimes(h, &created, &exited, &kernel, &user)
	return uint64(created.HighDateTime)<<32 | uint64(created.LowDateTime), err
}

func scratchOwnerAlive(owner scratchOwner) (bool, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, owner.PID)
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return false, nil // Windows reports this when the PID no longer exists.
	}
	if err != nil {
		return false, err // Access denied is unknown, never proof of exit.
	}
	defer windows.CloseHandle(h)
	created, err := processCreationTime(h)
	if err != nil || created != owner.Created {
		return false, err
	}
	state, err := windows.WaitForSingleObject(h, 0)
	return state == uint32(windows.WAIT_TIMEOUT), err
}
