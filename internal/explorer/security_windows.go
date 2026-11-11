package explorer

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

const supportedSecurity = windows.OWNER_SECURITY_INFORMATION | windows.GROUP_SECURITY_INFORMATION |
	windows.DACL_SECURITY_INFORMATION | windows.LABEL_SECURITY_INFORMATION

func readSecurity(f *os.File) (*windows.SECURITY_DESCRIPTOR, error) {
	// Microsoft SECURITY_INFORMATION: these parts need READ_CONTROL only.
	// Audit SACLs need ACCESS_SYSTEM_SECURITY and are invisible to ordinary
	// accounts, even through BackupRead. We do not enable privileges or claim
	// to preserve that inaccessible part (owner decision: D-0029).
	// https://learn.microsoft.com/windows/win32/secauthz/security-information
	flags := windows.SECURITY_INFORMATION(supportedSecurity | windows.ATTRIBUTE_SECURITY_INFORMATION | windows.SCOPE_SECURITY_INFORMATION)
	sd, err := windows.GetSecurityInfo(windows.Handle(f.Fd()), windows.SE_FILE_OBJECT, flags)
	if err != nil {
		return nil, err
	}
	if err := checkVisibleSecurity(sd); err != nil {
		return nil, err // Sync's caller records the reason in explorer.log.
	}
	return sd, nil
}

func checkVisibleSecurity(sd *windows.SECURITY_DESCRIPTOR) error {
	acl, _, err := sd.SACL()
	if errors.Is(err, windows.ERROR_OBJECT_NOT_FOUND) {
		return nil
	}
	if err != nil || acl == nil {
		return err
	}
	for i := uint16(0); i < acl.AceCount; i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, uint32(i), &ace); err != nil {
			return err
		}
		const systemMandatoryLabelACE = 0x11 // WinNT.h SYSTEM_MANDATORY_LABEL_ACE_TYPE
		if ace.Header.AceType != systemMandatoryLabelACE {
			return fmt.Errorf("desktop.ini has unsupported visible security ACE type %d (resource attribute or scope policy); leaving file unchanged", ace.Header.AceType)
		}
	}
	return nil
}

func securityBytes(sd *windows.SECURITY_DESCRIPTOR) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(sd)), sd.Length())
}

func copySecurity(sd *windows.SECURITY_DESCRIPTOR, target *os.File) error {
	flags := windows.SECURITY_INFORMATION(supportedSecurity)
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	group, _, err := sd.Group()
	if err != nil {
		return err
	}
	acl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	sacl, _, err := sd.SACL()
	if err != nil && !errors.Is(err, windows.ERROR_OBJECT_NOT_FOUND) {
		return err
	}
	control, _, err := sd.Control()
	if err != nil {
		return err
	}
	if control&windows.SE_DACL_PROTECTED != 0 {
		flags |= windows.PROTECTED_DACL_SECURITY_INFORMATION
	} else {
		flags |= windows.UNPROTECTED_DACL_SECURITY_INFORMATION
	}
	return windows.SetSecurityInfo(windows.Handle(target.Fd()), windows.SE_FILE_OBJECT, flags, owner, group, acl, sacl)
}
