//go:build windows

package testenv

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/pathutil"
	"golang.org/x/sys/windows"
)

const scratchOwnerFile = ".obit-test-owner.json"

type scratchOwner struct {
	PID     uint32 `json:"pid"`
	Created uint64 `json:"created"` // Windows FILETIME, in 100 ns units.
	Package string `json:"package"`
}

func writeScratchOwner(scratch, pkg string) error {
	created, err := processCreated()
	if err != nil {
		return err
	}
	data, err := json.Marshal(scratchOwner{uint32(os.Getpid()), created, pkg})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(scratch, scratchOwnerFile), data, 0o600)
}

func reapScratch(temp string, now time.Time, log io.Writer) {
	root, err := os.OpenRoot(temp)
	if err != nil {
		fmt.Fprintln(log, "test scratch recovery:", err)
		return
	}
	defer root.Close()
	entries, err := scratchEntries(root)
	if err != nil {
		fmt.Fprintln(log, "test scratch recovery:", err)
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !scratchName(entry.Name()) {
			continue
		}
		if err := reapScratchDirectory(root, entry.Name(), now); err != nil && !errors.Is(err, errScratchRecent) && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(log, "test scratch recovery: skipped %s: %v\n", filepath.Join(temp, entry.Name()), err)
		}
	}
}

func scratchName(name string) bool {
	digits, ok := strings.CutPrefix(name, "obit-tests-")
	return ok && digits != "" && strings.Trim(digits, "0123456789") == ""
}

func scratchPackage() string {
	pc, _, _, _ := runtime.Caller(2) // the package's TestMain, through Main
	return strings.TrimSuffix(runtime.FuncForPC(pc).Name(), ".TestMain")
}

var errScratchRecent = errors.New("scratch has recent writes")

func reapScratchDirectory(temp *os.Root, name string, now time.Time) error {
	info, err := temp.Lstat(name)
	if err != nil || !info.ModTime().Before(now.Add(-10*time.Minute)) {
		return err
	}
	if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return fmt.Errorf("not an ordinary directory")
	}
	// os.OpenRoot (unlike Root.OpenRoot) denies Windows rename/delete sharing.
	// Keep this candidate pinned until every removal inside it has finished.
	root, err := os.OpenRoot(filepath.Join(temp.Name(), name))
	if err != nil {
		return err
	}
	defer root.Close()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		return fmt.Errorf("scratch changed while opening: %w", errors.Join(err, os.ErrInvalid))
	}
	cutoff, err := scratchCutoff(root, now)
	if err != nil || cutoff.IsZero() {
		return err
	}
	acl, err := scratchCleanupACL(root.Name())
	if err != nil {
		return err
	}
	if err := prepareScratch(root.Name(), cutoff, acl, false); err != nil {
		return err
	}
	if err := emptyScratch(root); err != nil {
		return err
	}
	if err := root.Close(); err != nil {
		return err
	}
	// Only remove the now-empty leaf after releasing its non-delete-sharing
	// handle. Never recursively delete a name that could have been replaced.
	return temp.Remove(name)
}

func scratchCutoff(root *os.Root, now time.Time) (time.Time, error) {
	owner, err := readScratchOwner(root)
	if err != nil {
		return time.Time{}, err
	}
	if owner == nil {
		return now.Add(-24 * time.Hour), nil
	}
	alive, err := scratchOwnerAlive(*owner)
	if err != nil || alive {
		return time.Time{}, err
	}
	return now.Add(-10 * time.Minute), nil
}

func readScratchOwner(root *os.Root) (*scratchOwner, error) {
	info, err := root.Lstat(scratchOwnerFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 4096 {
		return nil, fmt.Errorf("invalid scratch owner record")
	}
	f, err := root.Open(scratchOwnerFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, fmt.Errorf("owner record changed: %w", errors.Join(err, os.ErrInvalid))
	}
	data, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil {
		return nil, err
	}
	var owner scratchOwner
	if err := json.Unmarshal(data, &owner); err != nil {
		return nil, err
	}
	if owner.PID == 0 || owner.Created == 0 || owner.Package == "" || len(data) > 4096 {
		return nil, fmt.Errorf("invalid scratch owner record")
	}
	return &owner, nil
}

// Go's os/root_openat.go and removeall_at.go provide handle-relative deletion.
// Use the standard library rather than implementing another Windows deleter.
func emptyScratch(root *os.Root) error {
	entries, err := scratchEntries(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() != scratchOwnerFile {
			if err := root.RemoveAll(entry.Name()); err != nil {
				return err
			}
		}
	}
	// Retain the owner record if a fixture cannot be removed.
	return root.RemoveAll(scratchOwnerFile)
}

func scratchEntries(root *os.Root) ([]os.DirEntry, error) {
	f, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.ReadDir(-1)
}

func prepareScratch(path string, cutoff time.Time, acl *windows.ACL, restore bool) error {
	h, info, err := openScratchEntry(path)
	if err != nil {
		return fmt.Errorf("open fixture ACL %s: %w", path, err)
	}
	defer windows.CloseHandle(h) // pins each ancestor during ACL restoration
	if !time.Unix(0, info.LastWriteTime.Nanoseconds()).Before(cutoff) {
		return errScratchRecent
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		if !restore {
			return fmt.Errorf("reparse point: %s", path)
		}
		// A fixture's link: os.Root removes the link itself, so neither its
		// target's ACL nor its contents are ever touched.
		return nil
	}
	// The candidate root is already readable and pinned by os.OpenRoot.
	if restore {
		if err := restoreScratchAccess(path, acl); err != nil {
			return fmt.Errorf("restore fixture ACL %s: %w", path, err)
		}
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		child := filepath.Join(path, entry.Name())
		if err := pathutil.Unchanged(child); err != nil {
			return err
		}
		// ReadDir already obtained leaf attributes without opening the file.
		// Restoring the parent grants FILE_DELETE_CHILD; changing a file's DACL
		// is unnecessary and could modify an external hard-link target.
		if !entry.IsDir() {
			if err := checkScratchLeaf(entry, cutoff); err != nil {
				return err
			}
			continue
		}
		if err := prepareScratch(child, cutoff, acl, true); err != nil {
			return err
		}
	}
	return nil
}

func checkScratchLeaf(entry os.DirEntry, cutoff time.Time) error {
	info, err := entry.Info()
	if err != nil {
		return err
	}
	// A link or special file is removed as an entry, never followed.
	if !info.ModTime().Before(cutoff) {
		return errScratchRecent
	}
	return nil
}
