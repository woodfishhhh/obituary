package winfs

import (
	"encoding/binary"
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// The CLI ships without cgo. SYS_PROC_INFO exposes the same kernel queries as
// libproc, including per-process permission errors that a filtered file list loses.
func checkOpenFiles(dev, ino uint64) error {
	procs, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return err
	}
	for _, proc := range procs {
		pid := int(proc.Proc.P_pid)
		if pid == 0 {
			continue
		} // kernel_task has no user file descriptor table.
		if err := inspectDarwinFDs(pid, dev, ino); err != nil && !errors.Is(err, unix.ESRCH) {
			return fmt.Errorf("inspect pid %d: %w", pid, err)
		}
	}
	return nil
}

func inspectDarwinFDs(pid int, dev, ino uint64) error {
	// proc_fdinfo is an int32 descriptor and a uint32 type. A full buffer may
	// have been truncated by a growing descriptor table, so repeat with room.
	for size := 256; ; size *= 2 {
		fds := make([]byte, size)
		n, err := procInfo(2, pid, 0, fds) // PROC_INFO_CALL_PIDINFO / PROC_PIDLISTFDS
		if err != nil {
			return err
		}
		if n > size || n%8 != 0 {
			return fmt.Errorf("invalid descriptor list size %d", n)
		}
		if n == size {
			continue
		}
		for offset := 0; offset < n; offset += 8 {
			if binary.NativeEndian.Uint32(fds[offset+4:]) != 1 {
				continue
			} // PROX_FDTYPE_VNODE
			fd := binary.NativeEndian.Uint32(fds[offset:])
			if err := inspectDarwinVnode(pid, fd, dev, ino); err != nil && !errors.Is(err, unix.EBADF) {
				return err
			}
		}
		return nil
	}
}

func inspectDarwinVnode(pid int, fd uint32, dev, ino uint64) error {
	// sys/proc_info.h: vnode_fdinfo starts with proc_fileinfo (24 bytes),
	// followed by vinfo_stat with vst_dev at offset 0 and vst_ino at offset 8.
	var info [176]byte
	n, err := procInfo(3, pid, fd, info[:]) // PROC_INFO_CALL_PIDFDINFO / PROC_PIDFDVNODEINFO
	if err != nil {
		return err
	}
	if n != len(info) {
		return fmt.Errorf("incomplete vnode information (%d bytes)", n)
	}
	if binary.NativeEndian.Uint32(info[24:]) == uint32(dev) && binary.NativeEndian.Uint64(info[32:]) == ino {
		return fmt.Errorf("%w at pid %d fd %d", errWatchFileOpen, pid, fd)
	}
	return nil
}

func procInfo(call uintptr, pid int, fd uint32, buf []byte) (int, error) {
	n, _, errno := syscall.Syscall6(unix.SYS_PROC_INFO, call, uintptr(pid), 1, uintptr(fd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if errno != 0 {
		return 0, errno
	}
	return int(n), nil
}
