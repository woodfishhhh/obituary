package pathutil

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
)

// Abs preserves a literal DOS-reserved leaf; every other component must be
// usable by the ordinary Win32 APIs without changing its meaning.
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
	if devicePath(filepath.FromSlash(path)) {
		return fmt.Errorf("device path %q is not a file; use a drive or share path", path)
	}
	if strings.Contains(strings.TrimPrefix(path, VolumeRoot(path)), ":") {
		return fmt.Errorf("%q names a stream; Obituary only handles whole files or directories", path)
	}
	if IsReservedLeaf(filepath.Base(path)) && ComparisonPath(path) == path {
		return fmt.Errorf("reserved Windows filename %q needs a literal local drive path; register the file again", path)
	}
	return nil
}

// Unchanged checks a spelling returned by the filesystem before it can be
// interpreted again as an ordinary Win32 path.
func Unchanged(path string) error {
	if plain := ComparisonPath(path); plain != path {
		if IsReservedLeaf(filepath.Base(path)) {
			_, err := Abs(path)
			return err
		}
		path = plain
	} else if strings.HasPrefix(path, `\\?\`) && len(path) >= 7 && path[5:7] == `:\` || IsReservedLeaf(filepath.Base(path)) {
		return normalizationError(path)
	}
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
	win := filepath.FromSlash(path)
	if devicePath(win) || strings.HasPrefix(win, `\\?\`) && !strings.HasPrefix(path, `\\?\`) {
		return "", fmt.Errorf("device path %q is not a file; use a drive or share path", path)
	}
	dir, leaf := filepath.Split(strings.TrimRight(win, `\`))
	for part := range strings.SplitSeq(dir[len(filepath.VolumeName(dir)):], `\`) {
		if IsReservedLeaf(part) {
			return "", fmt.Errorf("reserved Windows directory component %q cannot be used safely; use an ID", part)
		}
	}
	if IsReservedLeaf(leaf) {
		return reservedAbsolute(path, dir, leaf)
	}
	return ordinaryAbsolute(path)
}

func reservedAbsolute(path, dir, leaf string) (string, error) {
	if strings.ContainsAny(leaf, `:<>"|?*`) || strings.HasSuffix(leaf, ".") || strings.HasSuffix(leaf, " ") {
		return "", normalizationError(path)
	}
	if strings.HasPrefix(path, `\\?\`) && (strings.Contains(path, "/") || strings.HasSuffix(path, `\\`)) {
		return "", normalizationError(path)
	}
	// Resolve the same parent with a non-device leaf. GetFullPathNameW on NUL
	// itself can discard even an absolute drive-qualified parent.
	safe, err := ordinaryAbsolute(dir + "_" + leaf)
	if err != nil {
		return "", err
	}
	parent := filepath.Dir(safe)
	if len(filepath.VolumeName(parent)) != 2 || !filepath.IsAbs(parent) {
		return "", fmt.Errorf("reserved Windows filename %q requires a local drive path", leaf)
	}
	for p := parent; ; p = filepath.Dir(p) {
		if err := Unchanged(p); err != nil {
			return "", err
		}
		if p == filepath.Dir(p) {
			break
		}
	}
	return `\\?\` + filepath.Join(parent, leaf), nil
}

func devicePath(path string) bool {
	return strings.HasPrefix(path, `\\.\`) || strings.HasPrefix(path, `\??\`)
}

// IsReservedLeaf uses the standard library's Windows device table on the stem:
// Windows versions disagree on extensions, but the literal file must not vary.
func IsReservedLeaf(leaf string) bool {
	stem, _, _ := strings.Cut(leaf, ".")
	stem, _, _ = strings.Cut(stem, ":")
	stem = strings.TrimRight(stem, " ")
	return stem != "" && !strings.ContainsAny(stem, `\/`) && !filepath.IsLocal(stem)
}

// ComparisonPath removes the literal drive prefix for lexical comparisons only.
// Never pass its result to a filesystem API.
func ComparisonPath(path string) string {
	if !strings.HasPrefix(path, `\\?\`) || len(path) < 7 || path[5:7] != `:\` {
		return path
	}
	// A verbatim trailing dot/space can identify a different ordinary sibling.
	for part := range strings.SplitSeq(path[7:], `\`) {
		if strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return path
		}
	}
	return path[4:]
}

func ordinaryAbsolute(path string) (string, error) {
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
