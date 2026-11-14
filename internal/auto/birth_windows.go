package auto

import (
	"os"
	"syscall"
	"time"
)

// birthTime is when the file system created the object at path, not following
// a final link. Moving or renaming keeps it, which is why it backs up the
// session snapshot as evidence that the agent created something.
func birthTime(path string) (time.Time, bool) {
	info, err := os.Lstat(path)
	if err != nil {
		return time.Time{}, false
	}
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(0, data.CreationTime.Nanoseconds()), true
}
