package testenv_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/testenv"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"golang.org/x/sys/windows"
)

func TestScratchReapingPreservesSymlinkTargets(t *testing.T) {
	for _, kind := range []string{"root", "owned-child", "legacy-child", "owner-record"} {
		t.Run(kind, func(t *testing.T) { checkScratchLink(t, kind, false) })
	}
}

func TestScratchReapingPreservesJunctionTargets(t *testing.T) {
	if testing.Short() {
		t.Skip("creates junctions through a child process; symlink cases cover the fast tier")
	}
	for _, kind := range []string{"root", "owned-child", "legacy-child"} {
		t.Run(kind, func(t *testing.T) { checkScratchLink(t, kind, true) })
	}
}

func checkScratchLink(t *testing.T, kind string, junction bool) {
	t.Helper()
	temp := t.TempDir()
	old := time.Now().Add(-48 * time.Hour)
	owner := currentScratchOwner(t)
	owner.Created++
	outside := makeScratch(t, t.TempDir(), "outside", &owner, old)
	before := fixtureDACL(t, outside)
	fileBefore := fixtureDACL(t, filepath.Join(outside, "keep.txt"))
	root := filepath.Join(temp, "obit-tests-1")
	target, link := outside, root
	if kind != "root" {
		var record *testenv.ScratchOwner
		if kind == "owned-child" {
			record = &owner
		}
		makeScratch(t, temp, "obit-tests-1", record, old)
		link = filepath.Join(root, "link")
		if kind == "owner-record" {
			link = filepath.Join(root, testenv.ScratchOwnerFile)
			target = filepath.Join(outside, testenv.ScratchOwnerFile)
		}
	}
	if junction {
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
			t.Fatalf("junction: %s %v", out, err)
		}
	} else if err := os.Symlink(target, link); err != nil {
		if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
			t.Skip("Windows symlink privilege unavailable; junction cases exercise reparse protection")
		}
		t.Fatal(err)
	}
	if kind != "root" {
		ageLink(t, link, old)
		if err := os.Chtimes(root, old, old); err != nil {
			t.Fatal(err)
		}
	}
	var log bytes.Buffer
	testenv.ReapScratch(temp, time.Now(), &log)
	// A link inside the scratch goes with it, as a link; a linked scratch root
	// or owner record is never trusted.
	removed := kind == "owned-child" || kind == "legacy-child"
	if _, err := os.Lstat(root); os.IsNotExist(err) != removed {
		t.Fatalf("scratch removed=%v, want %v: %v; log=%s", os.IsNotExist(err), removed, err, &log)
	}
	if data, err := os.ReadFile(filepath.Join(outside, "keep.txt")); err != nil || string(data) != "fixture" {
		t.Fatalf("target changed: %q %v", data, err)
	}
	if got := fixtureDACL(t, outside); got != before {
		t.Fatalf("target directory DACL changed: %s => %s", before, got)
	}
	if got := fixtureDACL(t, filepath.Join(outside, "keep.txt")); got != fileBefore {
		t.Fatalf("target file DACL changed: %s => %s", fileBefore, got)
	}
}

// ageLink dates the link itself; os.Chtimes would date its target.
func ageLink(t *testing.T, path string, at time.Time) {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(name, windows.FILE_WRITE_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	ft := windows.NsecToFiletime(at.UnixNano())
	if err := windows.SetFileTime(h, &ft, &ft, &ft); err != nil {
		t.Fatal(err)
	}
}

func TestScratchReapingPreservesHardlinkTarget(t *testing.T) {
	temp := t.TempDir()
	root := makeScratch(t, temp, "obit-tests-1", nil, time.Now().Add(-48*time.Hour))
	outside := filepath.Join(t.TempDir(), "keep.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(outside, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(outside, filepath.Join(root, "linked.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(root, old, old); err != nil {
		t.Fatal(err)
	}
	before := fixtureDACL(t, outside)
	var log bytes.Buffer
	testenv.ReapScratch(temp, time.Now(), &log)
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("scratch survived: %v %s", err, &log)
	}
	if data, err := os.ReadFile(outside); err != nil || string(data) != "outside" {
		t.Fatalf("target changed: %q %v", data, err)
	}
	if got := fixtureDACL(t, outside); got != before {
		t.Fatalf("hardlink target ACL changed: %s => %s", before, got)
	}
}

func TestScratchReapingRestoresDeniedAccess(t *testing.T) {
	for _, mask := range []uint32{windows.FILE_LIST_DIRECTORY, windows.GENERIC_READ} {
		t.Run(fmt.Sprint(mask), func(t *testing.T) {
			temp := t.TempDir()
			root := makeScratch(t, temp, "obit-tests-1", nil, time.Now().Add(-48*time.Hour))
			denied := filepath.Join(root, "denied")
			makeScratch(t, root, "denied", nil, time.Now().Add(-48*time.Hour))
			ageScratch(t, root, time.Now().Add(-48*time.Hour))
			denyAbandonedFixture(t, denied, mask, false)
			if _, err := os.ReadDir(denied); err == nil {
				t.Fatal("fixture did not deny enumeration")
			}
			var log bytes.Buffer
			testenv.ReapScratch(temp, time.Now(), &log)
			if _, err := os.Stat(root); !os.IsNotExist(err) {
				t.Fatalf("denied scratch survived: %v %s", err, &log)
			}
		})
	}
}

func TestScratchReapingLogsDeniedRecovery(t *testing.T) {
	temp := t.TempDir()
	root := makeScratch(t, temp, "obit-tests-1", nil, time.Now().Add(-48*time.Hour))
	denied := makeScratch(t, root, "denied", nil, time.Now().Add(-48*time.Hour))
	ageScratch(t, root, time.Now().Add(-48*time.Hour))
	denyAbandonedFixture(t, denied, windows.WRITE_DAC, true)
	var log bytes.Buffer
	testenv.ReapScratch(temp, time.Now(), &log)
	if _, err := os.Stat(filepath.Join(root, "keep.txt")); err != nil {
		t.Fatal("partially deleted inaccessible scratch", err)
	}
	if !strings.Contains(log.String(), "skipped") || !strings.Contains(log.String(), denied) {
		t.Fatalf("missing permission diagnostic: %s", &log)
	}
}

func denyAbandonedFixture(t *testing.T, path string, mask uint32, denyOwner bool) {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	original, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(p, windows.WRITE_DAC|windows.READ_CONTROL, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		defer windows.CloseHandle(h)
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			return
		}
		if err := windows.SetSecurityInfo(h, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, original, nil); err != nil {
			t.Error(err)
		}
	})
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	extra := ""
	if denyOwner {
		extra = "(D;;WD;;;OW)"
	}
	denial, err := windows.SecurityDescriptorFromString(fmt.Sprintf("D:%s(D;;0x%x;;;%s)(A;;FA;;;%s)", extra, mask, u.User.Sid.String(), u.User.Sid.String()))
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := denial.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetSecurityInfo(h, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
}

func fixtureDACL(t *testing.T, path string) string {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	// Compare only the ACEs: on elevated CI runners the first query can carry
	// owner and group, and restoring sets the auto-inherited flag (AI).
	text := sd.String()
	if i := strings.Index(text, "D:"); i >= 0 {
		text = text[i+2:]
	}
	if i := strings.Index(text, "("); i >= 0 {
		text = text[i:]
	}
	return text
}

func TestScratchReapingKilledProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("kills a child test process; ownership and ACL cases cover the fast tier")
	}
	temp := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestScratchAbandonedChild$")
	cmd.Env = append(os.Environ(), "OBIT_TEST_ABANDON_SCRATCH=1", "TEMP="+temp, "TMP="+temp)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("child: %v %s", err, &stderr)
	}
	var root string
	if err := json.Unmarshal([]byte(line), &root); err != nil {
		t.Fatalf("child root: %v %s %s", err, line, &stderr)
	}
	var log bytes.Buffer
	testenv.ReapScratch(temp, time.Now(), &log)
	if _, err := os.Stat(root); err != nil {
		t.Fatal("live child scratch removed", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("killed child exited successfully")
	}
	recovery := exec.CommandContext(ctx, exe, "-test.run=^TestScratchMainRecordsOwner$")
	recovery.Env = cmd.Env
	if out, err := recovery.CombinedOutput(); err != nil {
		t.Fatalf("startup recovery: %s %v", out, err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("killed scratch survived: %v %s", err, &log)
	}
}

func TestScratchAbandonedChild(t *testing.T) {
	if testing.Short() {
		t.Skip("helper for killed-process recovery")
	}
	if os.Getenv("OBIT_TEST_ABANDON_SCRATCH") != "1" {
		t.Skip("subprocess helper")
	}
	data, err := userdirs.Data()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(data)
	denied := makeScratch(t, root, "denied", nil, time.Now().Add(-11*time.Minute))
	ageScratch(t, root, time.Now().Add(-11*time.Minute))
	testenv.DenyAccess(t, denied, windows.FILE_LIST_DIRECTORY)
	b, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(b))
	time.Sleep(time.Minute)
	t.Fatal("parent did not kill child")
}
