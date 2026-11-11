package winfs

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestObserveWatchBindsIdentityAndExcludesWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.zip")
	if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	fs := FS{}
	want, err := fs.Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	snap, release, err := fs.ObserveWatch(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if snap.Identity != want || snap.Size != 7 || snap.Modified.IsZero() {
		t.Fatalf("wrong watch snapshot: %+v", snap)
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(p, windows.GENERIC_WRITE, shareAll, nil, windows.OPEN_EXISTING, 0, 0)
	if err == nil {
		windows.CloseHandle(h)
		t.Fatal("watch hold allowed a new writer")
	}
	moved := path + ".moved"
	if err := fs.MovePinned(path, want, moved); err != nil {
		t.Fatal(err)
	}
}

func TestObserveWatchRejectsExistingWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.zip")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(p, windows.GENERIC_WRITE, shareAll, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	_, release, err := (FS{}).ObserveWatch(path, true)
	if err == nil {
		release()
		t.Fatal("watch hold ignored existing writer")
	}
}
