package winfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

func checkOpenFiles(dev, ino uint64) error {
	mounts, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return err
	}
	if !visibleProcessTable(string(mounts)) {
		return fmt.Errorf("cannot inspect the complete process table")
	}
	return eachProcNumber("/proc", func(pid string) error {
		// Threads may have private descriptor tables after unshare(CLONE_FILES).
		return eachProcNumber(filepath.Join(pid, "task"), func(tid string) error {
			return eachProcNumber(filepath.Join(tid, "fd"), func(fd string) error {
				var st unix.Stat_t
				if err := unix.Stat(fd, &st); err != nil {
					return err
				}
				if st.Dev == dev && st.Ino == ino {
					return fmt.Errorf("%w at %s", errWatchFileOpen, fd)
				}
				return nil
			})
		})
	})
}

func visibleProcessTable(mounts string) bool {
	for line := range strings.SplitSeq(mounts, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 || fields[4] != "/proc" || fields[len(fields)-3] != "proc" {
			continue
		}
		for option := range strings.SplitSeq(fields[len(fields)-1], ",") {
			if value, ok := strings.CutPrefix(option, "hidepid="); ok && value != "off" && value != "0" {
				return false
			}
		}
		return true
	}
	return false
}

func eachProcNumber(dir string, visit func(string) error) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if _, err := strconv.ParseUint(entry.Name(), 10, 32); err != nil {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if err := visit(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect %s: %w", path, err)
		}
	}
	return nil
}
