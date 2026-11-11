package regtxn

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"golang.org/x/sys/windows"
)

// Lock serializes installers in the user's protected data directory. Reuse
// core's LockFileEx wrapper: named kernel objects allow low-integrity squatting.
func Lock(resource string) (func(), error) {
	path, err := lockPath(resource)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return lockBefore(ctx, path)
}

func lockBefore(ctx context.Context, path string) (func(), error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("registry installation lock: %w", err)
		}
		release, ok, err := core.TryLockExclusive(path)
		if err != nil || ok {
			return release, err
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("registry installation lock: %w", ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func lockPath(resource string) (string, error) {
	dir, err := userdirs.RegistryLocks()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		return "", err
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", err
	}
	if err := protectLockDirectory(dir, user.User.Sid.String()); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(resource))
	return filepath.Join(dir, fmt.Sprintf("%x.lock", sum)), nil
}

func protectLockDirectory(dir, sid string) error {
	// os.Mkdir's mode does not set Windows ACLs. CreateDirectory installs the
	// DACL and mandatory label atomically; NR also prevents read-only lock DOS.
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;" + sid + ")S:(ML;OICI;NWNR;;;ME)")
	if err != nil {
		return err
	}
	name, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}
	attrs := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	if err := windows.CreateDirectory(name, &attrs); err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return err
	}
	attributes, err := windows.GetFileAttributes(name)
	if err != nil {
		return err
	}
	if attributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 || attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("registry lock directory must be an ordinary directory")
	}
	current, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.LABEL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	// Windows marks the SACL auto-inherited even when it contains only our label.
	if strings.Replace(current.String(), "S:AI(", "S:(", 1) != sd.String() {
		return fmt.Errorf("registry lock directory permissions changed; restore its private ACL")
	}
	return nil
}
