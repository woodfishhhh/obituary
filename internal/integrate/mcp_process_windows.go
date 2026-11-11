package integrate

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// runClaude uses a kill-on-close Job Object to terminate the full process tree.
// Go 1.26 does not expose PROC_THREAD_ATTRIBUTE_JOB_LIST in os/exec, so start
// suspended and assign before resuming: descendants cannot escape that gap.
func runClaude(cli string, args []string) ([]byte, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	closeJob := sync.OnceValue(func() error { return windows.CloseHandle(job) })
	defer closeJob()
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return nil, err
	}
	ctx, cancel := claudeMCPWithTimeout(context.Background(), claudeMCPTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, cli, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_SUSPENDED, HideWindow: true}
	cmd.WaitDelay = time.Second
	var jobMu sync.Mutex // cancellation must not close/reuse the handle during assignment
	cmd.Cancel = func() error {
		jobMu.Lock()
		defer jobMu.Unlock()
		// Kill also covers cancellation before the suspended process is assigned.
		_ = closeJob()
		return cmd.Process.Kill()
	}
	var output claudeOutput
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	startErr := func() error {
		jobMu.Lock()
		defer jobMu.Unlock()
		if err := ctx.Err(); err != nil {
			return err
		}
		h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
		if err != nil {
			return err
		}
		defer windows.CloseHandle(h)
		if err := windows.AssignProcessToJobObject(job, h); err != nil {
			return err
		}
		return resumeClaude(uint32(cmd.Process.Pid))
	}()
	if startErr != nil {
		_ = closeJob()
		_ = cmd.Process.Kill()
	}
	err = cmd.Wait()
	if ctx.Err() != nil {
		err = ctx.Err()
	} else if startErr != nil {
		err = startErr
	}
	return output.Bytes(), err
}

// os/exec closes CreateProcess's thread handle. The suspended process has
// exactly one thread; reopen it through the documented Toolhelp API.
func resumeClaude(pid uint32) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err := windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if err != nil {
			return err
		}
		defer windows.CloseHandle(thread)
		_, err = windows.ResumeThread(thread)
		return err
	}
	return fmt.Errorf("could not find suspended Claude thread for process %d", pid)
}
