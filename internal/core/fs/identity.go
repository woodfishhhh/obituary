package fs

import (
	"fmt"
	"runtime"
	"strings"
)

// RequirePersistent rejects POSIX records that contain only device and inode.
// Those values can be reused after deletion, even at the same pathname.
func (id Identity) RequirePersistent() error {
	if (runtime.GOOS == "linux" || runtime.GOOS == "darwin") && strings.HasPrefix(id.FileID, "0000000000000000") {
		return fmt.Errorf("POSIX identity lacks a persistent creation time/generation; cannot distinguish a deleted and recreated object or assign a safe lifespan; inspect the path and run obit mark <path> <lifespan> again; if the filesystem cannot supply it, use obit bury for an immediate move (later automatic recovery and purge are unavailable): %w", ErrUnsupported)
	}
	return nil
}

// Identity pins a tracked item to one file system object, so a renamed or
// moved item can be found again and a replaced path is never deleted by mistake.
type Identity struct {
	Volume string `json:"volume"` // volume root such as `C:\`
	Serial uint64 `json:"serial"` // volume serial number
	FileID string `json:"fileId"` // 128-bit file ID, hex
}

// Normalized makes volume spelling independent of the caller's path casing.
func (id Identity) Normalized() Identity {
	if runtime.GOOS != "windows" {
		return id
	}
	if strings.HasPrefix(strings.ToLower(id.Volume), `\\?\unc\`) {
		id.Volume = `\\` + id.Volume[8:]
	} else if strings.HasPrefix(id.Volume, `\\?\`) && len(id.Volume) >= 6 && id.Volume[5] == ':' {
		id.Volume = id.Volume[4:]
	}
	if len(id.Volume) >= 2 && id.Volume[1] == ':' {
		id.Volume = strings.ToUpper(id.Volume[:1]) + id.Volume[1:]
	} else {
		id.Volume = strings.ToLower(id.Volume)
	}
	return id
}
