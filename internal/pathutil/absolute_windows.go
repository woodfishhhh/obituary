package pathutil

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
)

// Abs accepts a verbatim spelling only when Win32's own normalizer preserves
// both it and its ordinary equivalent. Preserve verbatim names so normalization
// cannot redirect an operation to another object.
// syscall.FullPath delegates to GetFullPathNameW; no lexical approximation can
// reproduce its device-name and trailing-dot/space rules.
func Abs(path string) (string, error) {
	abs, err := absolute(path)
	if err != nil {
		return "", err
	}
	if err := WholeObject(abs); err != nil {
		return "", err
	}
	if RequiresDirectory(path) && !strings.HasSuffix(abs, `\`) {
		abs += `\`
	}
	return abs, nil
}

// RequiresDirectory preserves the trailing separator's object-type constraint.
// Volume roots already denote directories and need no extra marker.
func RequiresDirectory(path string) bool {
	return (strings.HasSuffix(path, `\`) || strings.HasSuffix(path, "/")) &&
		strings.TrimRight(path, `\/`) != strings.TrimRight(VolumeRoot(path), `\/`)
}

// WholeObject rejects named streams: a file ID identifies the host, not its ADS.
// Call on an absolute path, including when reading an older ledger entry.
func WholeObject(path string) error {
	if strings.Contains(strings.TrimPrefix(path, VolumeRoot(path)), ":") {
		return fmt.Errorf("%q names a stream; Obituary only handles whole files or directories", path)
	}
	return nil
}

// Unchanged checks a spelling returned by the filesystem before it can be
// interpreted again as an ordinary Win32 path.
func Unchanged(path string) error {
	full, err := syscall.FullPath(path)
	if err != nil {
		return err
	}
	if full != path {
		return normalizationError(path)
	}
	return nil
}

func absolute(path string) (string, error) {
	if !strings.HasPrefix(path, `\\?\`) {
		return filepath.Abs(path)
	}
	full, err := syscall.FullPath(path)
	if err != nil {
		return "", err
	}
	if full != path {
		return "", normalizationError(path)
	}
	plain := path[4:]
	if strings.HasPrefix(strings.ToUpper(plain), `UNC\`) {
		plain = `\\` + plain[4:]
	} else if len(filepath.VolumeName(plain)) != 2 || !filepath.IsAbs(plain) {
		// Volume GUID / GLOBALROOT names have no ordinary drive/share spelling.
		// Reject them rather than let later filepath operations reinterpret them.
		return "", fmt.Errorf("extended path %q is not a drive or UNC path; use an ID", path)
	}
	full, err = syscall.FullPath(plain)
	if err != nil {
		return "", err
	}
	if full != plain {
		return "", normalizationError(path)
	}
	return plain, nil
}

func normalizationError(path string) error {
	return fmt.Errorf("Windows normalization changes %q into another name; this spelling cannot be used safely; use an ID", path) //nolint:staticcheck // ST1005: Windows is a proper noun; path and init tests depend on this diagnostic.
}
