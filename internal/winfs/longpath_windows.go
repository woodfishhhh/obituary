package winfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"golang.org/x/sys/windows"
)

// FileNormalizedNameInformation works on the opened object without listing its
// parent (NtQueryInformationFile, Microsoft ntifs.h reference). Query only the
// final component, opened without following reparse points, and keep the
// lexical parent. GetLongPathNameW repeats directory enumeration at each depth;
// GetFinalPathNameByHandle would also replace lexical junction locations.
func (FS) LongPath(path string) (string, bool, error) {
	abs, err := pathutil.Abs(path)
	if err != nil {
		return "", false, fmt.Errorf("%v: %w", err, corefs.ErrUnsafe) //nolint:errorlint // Path validation exposes only ErrUnsafe, not implementation errors.
	}
	var original *corefs.Identity
	if id, err := (FS{}).Identify(filepath.Clean(abs)); err == nil {
		original = &id
	} else if !errors.Is(err, corefs.ErrNotFound) {
		return "", false, err
	}
	expanded, complete, err := expandLongPath(abs)
	if err != nil {
		return "", false, err
	}
	if err := unchangedPrefixes(expanded); err != nil {
		return "", false, fmt.Errorf("%v: %w", err, corefs.ErrUnsafe) //nolint:errorlint // Path validation exposes only ErrUnsafe, not implementation errors.
	}
	if original != nil {
		got, err := (FS{}).Identify(expanded)
		if err != nil || got != *original {
			return "", false, fmt.Errorf("expanded path %q no longer identifies %q; use an ID: %w", expanded, path, corefs.ErrUnsafe)
		}
	}
	if pathutil.RequiresDirectory(path) {
		info, err := os.Lstat(expanded)
		if err == nil && !info.IsDir() {
			return "", false, fmt.Errorf("%s is not a directory: %w", path, corefs.ErrUnsafe)
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", false, err
		}
		if !strings.HasSuffix(expanded, `\`) {
			expanded += `\`
		}
	}
	return expanded, complete, nil
}

// Every prefix is later used independently (for example captureParents calls
// filepath.Dir). GetFullPathNameW can preserve the full path while changing
// one of those parents, so the filesystem boundary must check all of them.
func unchangedPrefixes(path string) error {
	for {
		if err := pathutil.Unchanged(path); err != nil {
			return err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return nil
		}
		path = parent
	}
}

func expandLongPath(path string) (string, bool, error) {
	prefix := pathutil.VolumeRoot(path)
	parts := strings.Split(strings.TrimPrefix(path, prefix), string(filepath.Separator))
	root, err := windows.UTF16PtrFromString(prefix)
	if err != nil {
		return "", false, err
	}
	parent, err := windows.CreateFile(root, windows.FILE_TRAVERSE|windows.FILE_READ_ATTRIBUTES, shareAll, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", false, err
	}
	defer func() {
		if parent != 0 {
			windows.CloseHandle(parent)
		}
	}()
	complete := true
	buf := make([]byte, 4+32768*2)
	for i, part := range parts {
		if part == "" {
			continue
		}
		prefix = filepath.Join(prefix, part)
		var h windows.Handle
		access := uint32(windows.FILE_READ_ATTRIBUTES)
		if i+1 < len(parts) {
			access |= windows.FILE_TRAVERSE
		}
		if parent != 0 {
			n, nameErr := windows.NewNTUnicodeString(part)
			if nameErr != nil {
				return "", false, nameErr
			}
			attrs := windows.OBJECT_ATTRIBUTES{RootDirectory: parent, ObjectName: n, Attributes: windows.OBJ_CASE_INSENSITIVE}
			attrs.Length = uint32(unsafe.Sizeof(attrs))
			err = windows.NtCreateFile(&h, access, &attrs, &windows.IO_STATUS_BLOCK{}, nil, windows.FILE_ATTRIBUTE_NORMAL, shareAll, windows.FILE_OPEN, windows.FILE_OPEN_REPARSE_POINT|windows.FILE_OPEN_FOR_BACKUP_INTENT, 0, 0)
			var status windows.NTStatus
			if errors.As(err, &status) {
				err = status.Errno()
			}
		} else {
			n, _ := windows.UTF16PtrFromString(longPath(prefix))
			h, err = windows.CreateFile(n, access, shareAll, nil, windows.OPEN_EXISTING, openFlags, 0)
		}
		if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
			return filepath.Join(append([]string{prefix}, parts[i+1:]...)...), complete, nil
		}
		if parent != 0 {
			windows.CloseHandle(parent)
			parent = 0
		}
		if err == nil {
			parent = h
			name, queryErr := normalizedComponent(h, buf)
			if queryErr == nil {
				prefix = filepath.Join(filepath.Dir(prefix), name)
			} else {
				err = queryErr
			}
		}
		if err != nil {
			// An unexpanded ordinary spelling may be retained, but it must not
			// claim longNames. A plausible short alias is not a stable location.
			if strings.Contains(part, "~") {
				return "", false, fmt.Errorf("cannot expand possible short-name component %s; use its long name: %w", prefix, err)
			}
			complete = false
		}
		if parent != 0 && i+1 < len(parts) {
			var info windows.ByHandleFileInformation
			if err := windows.GetFileInformationByHandle(parent, &info); err != nil {
				return "", false, err
			}
			if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
				windows.CloseHandle(parent)
				parent = 0
				n, _ := windows.UTF16PtrFromString(longPath(prefix))
				parent, err = windows.CreateFile(n, windows.FILE_TRAVERSE|windows.FILE_READ_ATTRIBUTES, shareAll, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
				if err != nil {
					return "", false, err
				}
			}
		}
	}
	return filepath.Clean(prefix), complete, nil
}

func normalizedComponent(h windows.Handle, buf []byte) (string, error) {
	const fileNormalizedNameInformation = 48
	err := windows.NtQueryInformationFile(h, &windows.IO_STATUS_BLOCK{}, &buf[0], uint32(len(buf)), fileNormalizedNameInformation)
	if errors.Is(err, windows.STATUS_INVALID_INFO_CLASS) || errors.Is(err, windows.STATUS_NOT_SUPPORTED) || errors.Is(err, windows.STATUS_INVALID_PARAMETER) {
		// SMB does not implement normalized-name queries. The documented
		// GetFinalPathNameByHandle fallback may need ancestor access there;
		// failures stay incomplete (or refuse aliases) in the caller.
		wide := unsafe.Slice((*uint16)(unsafe.Pointer(&buf[4])), (len(buf)-4)/2)
		n, finalErr := windows.GetFinalPathNameByHandle(h, &wide[0], uint32(len(wide)), 4 /* VOLUME_NAME_NONE */)
		if finalErr != nil {
			return "", finalErr
		}
		if n >= uint32(len(wide)) {
			return "", windows.ERROR_FILENAME_EXCED_RANGE
		}
		return filepath.Base(windows.UTF16ToString(wide[:n])), nil
	}
	if err != nil {
		return "", err
	}
	n := *(*uint32)(unsafe.Pointer(&buf[0]))
	if n == 0 || n%2 != 0 || n > uint32(len(buf)-4) {
		return "", windows.ERROR_INVALID_DATA
	}
	name := windows.UTF16ToString(unsafe.Slice((*uint16)(unsafe.Pointer(&buf[4])), int(n/2)))
	return filepath.Base(name), nil
}
