// Package winfs implements core.FS with Windows file IDs and the shell's Recycle Bin.
package winfs

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"golang.org/x/sys/windows"
)

// FS is the Windows implementation of core.FS.
type FS struct{}

var _ core.FS = FS{}

type fileIDInfo struct {
	VolumeSerialNumber uint64
	FileID             [16]byte
}

// fileIDDescriptor mirrors FILE_ID_DESCRIPTOR with an ExtendedFileIdType ID.
type fileIDDescriptor struct {
	Size   uint32
	Type   uint32
	FileID [16]byte
}

const (
	extendedFileIDType = 2
	fileNameNormalized = 0x0
	volumeNameDOS      = 0x0
	shareAll           = windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE
	openFlags          = windows.FILE_FLAG_BACKUP_SEMANTICS | windows.FILE_FLAG_OPEN_REPARSE_POINT
)

var (
	kernel32         = windows.NewLazySystemDLL("kernel32.dll")
	procOpenFileByID = kernel32.NewProc("OpenFileById")
)

func open(path string) (windows.Handle, error) {
	return openWithFlags(path, openFlags)
}

func openFollowing(path string) (windows.Handle, error) {
	return openWithFlags(path, windows.FILE_FLAG_BACKUP_SEMANTICS)
}

func openWithFlags(path string, flags uint32) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES, shareAll, nil, windows.OPEN_EXISTING, flags, 0)
}

// IdentifyFollowing identifies the target, including the final reparse point.
// Unlike core.FS.Identify, this inspection permits multiple hard links. Compare
// Serial and FileID, not Volume: drive, UNC and mount aliases name the same ID.
func IdentifyFollowing(path string) (core.Identity, error) {
	h, err := openFollowing(path)
	if err != nil {
		return core.Identity{}, err
	}
	defer windows.CloseHandle(h)
	id, err := queryID(h)
	if err != nil {
		return core.Identity{}, err
	}
	return core.Identity{Volume: volumeRoot(path), Serial: id.VolumeSerialNumber, FileID: hex.EncodeToString(id.FileID[:])}, nil
}

func (FS) IdentifyFollowing(path string) (core.Identity, error) { return IdentifyFollowing(path) }

func (FS) Resolve(path string) (string, error) { return Resolve(path) }

func queryID(h windows.Handle) (fileIDInfo, error) {
	var info fileIDInfo
	err := windows.GetFileInformationByHandleEx(h, windows.FileIdInfo, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	return info, err
}

func volumeRoot(path string) string { return pathutil.VolumeRoot(path) }

// Identify returns the volume serial and 128-bit file ID of path without
// following links. Files with more than one hard link are refused: following
// such an ID could reach a name that was never registered.
func (FS) Identify(path string) (core.Identity, error) {
	h, err := open(path)
	if err != nil {
		return core.Identity{}, mapNotFound(err)
	}
	defer windows.CloseHandle(h)
	return identify(h, volumeRoot(path))
}

// IdentifyHandle reads the identity from an already pinned object. Path only
// supplies its volume spelling, as in FS.Identify.
func IdentifyHandle(h windows.Handle, path string) (core.Identity, error) {
	return identify(h, volumeRoot(path))
}

func identify(h windows.Handle, volume string) (core.Identity, error) {
	var basic windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &basic); err != nil {
		return core.Identity{}, err
	}
	if basic.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 && basic.NumberOfLinks > 1 {
		return core.Identity{}, fmt.Errorf("it has %d hard links: %w", basic.NumberOfLinks, core.ErrUnsafe)
	}
	info, err := queryID(h)
	if err != nil {
		return core.Identity{}, err
	}
	return (core.Identity{Volume: volume, Serial: info.VolumeSerialNumber, FileID: hex.EncodeToString(info.FileID[:])}).Normalized(), nil
}

// Locate opens the object by its file ID and returns its current path. The
// path is verified to lead back to the same object. A changed name retries;
// only unsafe properties of the object itself (such as hard links) refuse it.
func (fs FS) Locate(id core.Identity) (string, error) {
	return locate(id, fs.Identify)
}

func locate(id core.Identity, identifyPath func(string) (core.Identity, error)) (string, error) {
	id = id.Normalized()
	raw, err := hex.DecodeString(id.FileID)
	if err != nil || len(raw) != 16 {
		return "", fmt.Errorf("bad file ID %q", id.FileID)
	}
	root, err := open(id.Volume)
	if err != nil {
		return "", fmt.Errorf("volume %s is not available: %w", id.Volume, err) // unknown, not gone
	}
	defer windows.CloseHandle(root)
	if info, err := queryID(root); err != nil || info.VolumeSerialNumber != id.Serial {
		return "", fmt.Errorf("volume %s is not the one the item was on", id.Volume)
	}
	desc := fileIDDescriptor{Size: uint32(unsafe.Sizeof(fileIDDescriptor{})), Type: extendedFileIDType}
	copy(desc.FileID[:], raw)
	r, _, callErr := procOpenFileByID.Call(uintptr(root), uintptr(unsafe.Pointer(&desc)),
		windows.FILE_READ_ATTRIBUTES, shareAll, 0, openFlags)
	h := windows.Handle(r)
	if h == windows.InvalidHandle {
		return "", mapNotFound(callErr)
	}
	defer windows.CloseHandle(h)
	if _, err := identify(h, id.Volume); err != nil {
		return "", err
	}
	path, err := finalPath(h)
	if err != nil {
		return "", err
	}
	back, err := identifyPath(path)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			// The ID is still open: losing its name during a concurrent rename
			// is not proof that the object disappeared.
			return "", fmt.Errorf("reopen current name %q: %v", path, err) //nolint:errorlint // Exposing ErrNotFound would let callers discard a still-open object.
		}
		return "", fmt.Errorf("reopen current name %q: %w", path, err)
	}
	if back.Normalized() != id {
		return "", fmt.Errorf("its current name %q changed while locating the object; try again", path)
	}
	return path, nil
}

// Canonical returns the real location of path: short names expanded and
// links in parent folders resolved. The final component is not followed.
func Canonical(path string) (string, error) {
	return canonical(path, open)
}

// Resolve follows every reparse point, including the final component. Keep
// Canonical's no-follow-final contract for core's pinned filesystem operations.
func Resolve(path string) (string, error) {
	return canonical(path, openFollowing)
}

func canonical(path string, open func(string) (windows.Handle, error)) (string, error) {
	h, err := open(path)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	return checkedFinalPath(h, open)
}

// checkedFinalPath verifies that removing the long-path prefix still names the
// object held by h. It also serves receipts while a renamed source stays open.
func checkedFinalPath(h windows.Handle, open func(string) (windows.Handle, error)) (string, error) {
	want, err := queryID(h)
	if err != nil {
		return "", err
	}
	path, err := finalPath(h)
	if err != nil {
		return "", err
	}
	back, err := open(path)
	if err != nil {
		if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
			return "", fmt.Errorf("its current name %q cannot be addressed safely: %w", path, core.ErrUnsafe)
		}
		return "", err
	}
	defer windows.CloseHandle(back)
	// Compare only identity: Canonical also accepts multiply linked files.
	got, err := queryID(back)
	if err != nil {
		return "", err
	}
	if got != want {
		return "", fmt.Errorf("its current name %q cannot be addressed safely: %w", path, core.ErrUnsafe)
	}
	return path, nil
}

func finalPath(h windows.Handle) (string, error) {
	buf := make([]uint16, windows.MAX_PATH)
	for {
		n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), fileNameNormalized|volumeNameDOS)
		if err != nil {
			return "", err
		}
		if int(n) < len(buf) {
			return trimLongPrefix(windows.UTF16ToString(buf[:n])), nil
		}
		buf = make([]uint16, n+1)
	}
}

func trimLongPrefix(p string) string {
	if strings.HasPrefix(p, `\\?\UNC\`) {
		return `\\` + p[len(`\\?\UNC\`):]
	}
	return strings.TrimPrefix(p, `\\?\`)
}

func mapNotFound(err error) error {
	switch {
	case errors.Is(err, windows.ERROR_FILE_NOT_FOUND),
		errors.Is(err, windows.ERROR_PATH_NOT_FOUND),
		errors.Is(err, windows.ERROR_INVALID_PARAMETER),
		errors.Is(err, windows.ERROR_INVALID_NAME):
		return core.ErrNotFound
	}
	return err
}
