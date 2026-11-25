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

func checkOpenFiles(dev, ino uint64, owner uint32, private bool) error {
	mounts, err := os.ReadFile("/proc/self/mountinfo")
	if err == nil && !visibleProcessTable(string(mounts)) {
		err = fmt.Errorf("cannot inspect the complete process table")
	}
	first := err
	result := eachProcNumber("/proc", func(pid string) error {
		// Threads may have private descriptor tables after unshare(CLONE_FILES).
		tasks := filepath.Join(pid, "task")
		entries, err := os.ReadDir(tasks)
		if err != nil {
			return linuxWatchGap(pid, owner, private, err)
		}
		return visitProcNumbers(tasks, entries, func(tid string) error {
			err := eachProcNumber(filepath.Join(tid, "fd"), func(fd string) error {
				var st unix.Stat_t
				if err := unix.Stat(fd, &st); err != nil {
					return err
				}
				if st.Dev == dev && st.Ino == ino {
					return fmt.Errorf("%w at %s", errWatchFileOpen, fd)
				}
				return nil
			})
			return linuxWatchGap(tid, owner, private, err)
		})
	})
	recordWatchError(&first, result)
	return first
}

func linuxWatchGap(tid string, owner uint32, private bool, err error) error {
	if err == nil || !private || errors.Is(err, errWatchFileOpen) {
		return err
	}
	// proc directory ownership can become root for a non-dumpable user process.
	data, readErr := os.ReadFile(filepath.Join(tid, "status"))
	if readErr != nil {
		return err
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if !strings.HasPrefix(line, "Uid:") {
			continue
		}
		ids := strings.Fields(line)
		if len(ids) != 5 {
			return err
		}
		for _, text := range ids[1:] {
			uid, e := strconv.ParseUint(text, 10, 32)
			if e != nil || uint32(uid) == owner {
				return err
			}
		}
		return nil
	}
	return err
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
	return visitProcNumbers(dir, entries, visit)
}

func visitProcNumbers(dir string, entries []os.DirEntry, visit func(string) error) error {
	var first error
	for _, entry := range entries {
		if _, err := strconv.ParseUint(entry.Name(), 10, 32); err != nil {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if err := visit(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			if recordWatchError(&first, fmt.Errorf("inspect %s: %w", path, err)) {
				break
			}
		}
	}
	return first
}
