package pathutil

import (
	"path/filepath"
	"strings"
)

// VolumeRoot includes the host and share for extended UNC. Go 1.26's
// internal/filepathlite/path_windows.go volumeNameLen includes them only for
// \\.\UNC; use the ordinary UNC parser and put the verbatim prefix back.
func VolumeRoot(path string) string {
	if strings.HasPrefix(strings.ToLower(path), `\\?\unc\`) {
		return path[:8] + strings.TrimPrefix(filepath.VolumeName(`\\`+path[8:]), `\\`) + `\`
	}
	return filepath.VolumeName(path) + `\`
}
