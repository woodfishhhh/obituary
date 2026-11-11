package testenv

import (
	"fmt"
	"testing"

	"golang.org/x/sys/windows"
)

// DenyAccess changes only the named disposable fixture, never its children.
// Keep WRITE_DAC available so cleanup can restore the original permissions.
func DenyAccess(t *testing.T, path string, mask uint32) {
	t.Helper()
	original, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	originalACL, _, err := original.DACL()
	if err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sid := user.User.Sid.String()
	denied, err := windows.SecurityDescriptorFromString(fmt.Sprintf("D:(D;;0x%x;;;%s)(A;;FA;;;%s)", mask, sid, sid))
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := denied.DACL()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := restoreFixtureAccess(path, originalACL); err != nil {
			t.Errorf("restore fixture ACL %s: %v", path, err)
		}
	})
	if err := restoreFixtureAccess(path, acl); err != nil {
		t.Fatal(err)
	}
}

func restoreFixtureAccess(path string, acl *windows.ACL) error {
	h, info, err := openFixtureHandle(path, windows.WRITE_DAC|windows.READ_CONTROL|windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("fixture is a link: %s", path)
	}
	return restoreFixtureDACL(h, acl, false)
}

func restoreFixtureDACL(h windows.Handle, acl *windows.ACL, protected bool) error {
	flags := windows.DACL_SECURITY_INFORMATION
	if protected {
		flags |= windows.PROTECTED_DACL_SECURITY_INFORMATION
	}
	return windows.SetSecurityInfo(h, windows.SE_FILE_OBJECT, windows.SECURITY_INFORMATION(flags), nil, nil, acl, nil)
}

// SetSecurityInfo normally rewrites inherited ACLs throughout the subtree,
// including unreadable children. Deny sharing reads while changing this one
// DACL to suppress propagation, as documented by Microsoft; the metadata-only
// handle in prepareScratch keeps the name pinned after this handle closes.
// https://learn.microsoft.com/windows/win32/api/aclapi/nf-aclapi-setsecurityinfo
func restoreScratchAccess(path string, acl *windows.ACL) error {
	h, _, err := openFixtureHandle(path, windows.WRITE_DAC|windows.READ_CONTROL, windows.FILE_SHARE_WRITE)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return restoreFixtureDACL(h, acl, true)
}
