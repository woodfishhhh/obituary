package auto

import (
	"time"

	"golang.org/x/sys/unix"
)

// birthTime is when the file system created the object at path, not following
// a final link (see birth_windows.go). File systems or kernels without statx
// birth times report false, and nothing there counts as created by the agent.
func birthTime(path string) (time.Time, bool) {
	var st unix.Statx_t
	if err := unix.Statx(unix.AT_FDCWD, path, unix.AT_SYMLINK_NOFOLLOW, unix.STATX_BTIME, &st); err != nil || st.Mask&unix.STATX_BTIME == 0 {
		return time.Time{}, false
	}
	return time.Unix(st.Btime.Sec, int64(st.Btime.Nsec)), true
}
