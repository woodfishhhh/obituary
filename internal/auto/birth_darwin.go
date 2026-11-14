package auto

import (
	"os"
	"syscall"
	"time"
)

// birthTime is when the file system created the object at path, not following
// a final link (see birth_windows.go).
func birthTime(path string) (time.Time, bool) {
	info, err := os.Lstat(path)
	if err != nil {
		return time.Time{}, false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(st.Birthtimespec.Sec, st.Birthtimespec.Nsec), true
}
