package fs

import (
	"runtime"
	"strings"
)

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
