package regtxn_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/woodfishhhh/obituary/internal/regtxn"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func TestRegistryLockIgnoresLowIntegritySquat(t *testing.T) {
	if testing.Short() {
		t.Skip("old mutex regression waits for its 10 second deadline")
	}
	for _, kind := range []string{"mutex", "event"} {
		t.Run(kind, func(t *testing.T) {
			resource := "obituary-test-" + rand.Text()
			user, err := windows.GetCurrentProcessToken().GetTokenUser()
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256([]byte(user.User.Sid.String() + "\x00" + resource))
			name := windows.StringToUTF16Ptr(fmt.Sprintf(`Global\Obituary.Registry.%x`, sum))
			withLowIntegrity(t, func() {
				var h windows.Handle
				if kind == "mutex" {
					h, err = windows.CreateMutex(nil, true, name)
				} else {
					h, err = windows.CreateEvent(nil, 0, 0, name)
				}
				if err != nil {
					t.Fatal(err)
				}
				defer windows.CloseHandle(h)
				if kind == "mutex" {
					defer windows.ReleaseMutex(h)
				}
				done := make(chan error, 1)
				go func() {
					unlock, err := regtxn.Lock(resource)
					if err == nil {
						unlock()
					}
					done <- err
				}()
				if err := <-done; err != nil {
					t.Fatalf("low integrity squat blocked registry lock: %v", err)
				}
			})
		})
	}
}

func withLowIntegrity(t *testing.T, fn func()) {
	t.Helper()
	var original, low windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY, &original); err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	if err := windows.DuplicateTokenEx(original, windows.TOKEN_ALL_ACCESS, nil, windows.SecurityImpersonation, windows.TokenImpersonation, &low); err != nil {
		t.Fatal(err)
	}
	defer low.Close()
	sid, err := windows.StringToSid("S-1-16-4096")
	if err != nil {
		t.Fatal(err)
	}
	label := windows.Tokenmandatorylabel{Label: windows.SIDAndAttributes{Sid: sid, Attributes: windows.SE_GROUP_INTEGRITY}}
	if err := windows.SetTokenInformation(low, windows.TokenIntegrityLevel, (*byte)(unsafe.Pointer(&label)), label.Size()); err != nil {
		t.Fatal(err)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.SetThreadToken(nil, low); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := windows.RevertToSelf(); err != nil {
			t.Fatal(err)
		}
	}()
	fn()
}

func TestRegistryLockRejectsLowIntegrityFileAccess(t *testing.T) {
	resource := "obituary-test-" + rand.Text()
	unlock, err := regtxn.Lock(resource)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
	dir, err := userdirs.RegistryLocks()
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(resource))
	path := filepath.Join(dir, fmt.Sprintf("%x.lock", sum))
	withLowIntegrity(t, func() {
		for _, flags := range []int{os.O_RDONLY, os.O_RDWR} {
			f, err := os.OpenFile(path, flags, 0o600)
			if err == nil {
				f.Close()
			}
			if !errors.Is(err, os.ErrPermission) {
				t.Fatalf("low integrity opened registry lock: %v", err)
			}
		}
		f, err := os.OpenFile(filepath.Join(dir, "obituary-test-squat.lock"), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
		if err == nil {
			f.Close()
		}
		if !errors.Is(err, os.ErrPermission) {
			t.Fatalf("low integrity created registry lock: %v", err)
		}
	})
}

func TestRegistryLockHasBoundedWait(t *testing.T) {
	if testing.Short() {
		t.Skip("exercises the 10 second lock deadline")
	}
	resource := "obituary-test-" + rand.Text()
	unlock, err := regtxn.Lock(resource)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	done := make(chan error, 1)
	go func() {
		release, err := regtxn.Lock(resource)
		if err == nil {
			release()
		}
		done <- err
	}()
	if err := <-done; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("registry lock lost bounded wait: %v", err)
	}
}

func TestRegistryLockSharedAcrossDataHomes(t *testing.T) {
	t.Setenv("OBIT_HOME", t.TempDir())
	root := "obituary-test-" + rand.Text()
	release, err := regtxn.Lock(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("OBIT_HOME", t.TempDir())
	assertConcurrentInstallerWaits(t, root, release)
}

func TestRegistryLockExcludesConcurrentInstaller(t *testing.T) {
	root := testenv.ClassesKey(t)
	release, err := regtxn.Lock(root)
	if err != nil {
		t.Fatal(err)
	}
	assertConcurrentInstallerWaits(t, root, release)
}

func assertConcurrentInstallerWaits(t *testing.T, root string, release func()) {
	t.Helper()
	started := make(chan struct{})
	acquired := make(chan error, 1)
	go func() {
		close(started)
		unlock, err := regtxn.Lock(root)
		if err == nil {
			unlock()
		}
		acquired <- err
	}()
	<-started
	select {
	case err := <-acquired:
		release()
		t.Fatalf("registry lock allowed overlapping installers: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	release()
	if err := <-acquired; err != nil {
		t.Fatal(err)
	}
}
