package contextmenu

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestDropTargetReceivesFiftyFilesAndFolders(t *testing.T) {
	root := t.TempDir()
	var paths []string
	for i := range 50 {
		path := filepath.Join(root, fmt.Sprintf("item %d & 中文 😀", i))
		var err error
		if i%2 == 0 {
			err = os.Mkdir(path, 0700)
		} else {
			err = os.WriteFile(path, []byte("keep"), 0600)
		}
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	err := withWinRT(func() error {
		data := selectionData(t, root, paths)
		defer data.release()
		d := &dropTarget{vtable: &targetVtable, released: make(chan struct{}), selection: make(chan dropSelection, 1)}
		factory := &dropFactory{vtable: &factoryVtable, target: d}
		defer runtime.KeepAlive(factory)
		var object *runtimeObject
		hr := (*runtimeObject)(unsafe.Pointer(factory)).call(3, 0, uintptr(unsafe.Pointer(&iidDrop)), uintptr(unsafe.Pointer(&object)))
		if err := winrtError(hr, "create DropTarget"); err != nil {
			return err
		}
		var effect uint32 = 3
		args := []uintptr{uintptr(unsafe.Pointer(object)), uintptr(unsafe.Pointer(data)), 0, 0}
		if runtime.GOARCH == "386" {
			args = append(args, 0)
		}
		args = append(args, uintptr(unsafe.Pointer(&effect)))
		hr, _, _ = syscall.SyscallN(object.vtable[6], args...)
		object.release()
		if err := winrtError(hr, "drop selection"); err != nil {
			return err
		}
		got := <-d.selection
		if got.err != nil || !reflect.DeepEqual(got.paths, paths) || effect != 0 {
			t.Fatalf("paths=%q err=%v effect=%d", got.paths, got.err, effect)
		}
		select {
		case <-d.released:
		default:
			t.Fatal("COM client reference leaked")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Build a real Shell data object in memory; no Explorer or desktop process runs.
func selectionData(t *testing.T, root string, paths []string) *runtimeObject {
	t.Helper()
	parse := shell32.NewProc("SHParseDisplayName")
	parsePath := func(path string) unsafe.Pointer {
		name, err := windows.UTF16PtrFromString(path)
		if err != nil {
			t.Fatal(err)
		}
		var pidl unsafe.Pointer
		hr, _, _ := parse.Call(uintptr(unsafe.Pointer(name)), 0, uintptr(unsafe.Pointer(&pidl)), 0, 0)
		if err := winrtError(hr, "parse selection path"); err != nil {
			t.Fatal(err)
		}
		return pidl
	}
	parent := parsePath(root)
	defer windows.CoTaskMemFree(parent)
	children := make([]unsafe.Pointer, len(paths))
	for i, path := range paths {
		absolute := parsePath(path)
		defer windows.CoTaskMemFree(absolute)
		// ITEMIDLIST entries carry their own size; the final nonzero entry is
		// the child relative to the selected files' shared parent.
		child := absolute
		for next := unsafe.Add(child, *(*uint16)(child)); *(*uint16)(next) != 0; next = unsafe.Add(child, *(*uint16)(child)) {
			child = next
		}
		children[i] = child
	}
	iidData := windows.GUID{Data1: 0x10e, Data4: iidUnknown.Data4}
	var data *runtimeObject
	hr, _, _ := shell32.NewProc("SHCreateDataObject").Call(uintptr(parent), uintptr(len(children)), uintptr(unsafe.Pointer(&children[0])), 0, uintptr(unsafe.Pointer(&iidData)), uintptr(unsafe.Pointer(&data)))
	if err := winrtError(hr, "create selection data object"); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDropServerRejectsInvalidActivation(t *testing.T) {
	for _, args := range [][]string{nil, {`C:\obit.exe`, "purge"}, {"relative.exe", "1d"}} {
		if err := RunDropServer(args, nil, nil); err == nil {
			t.Fatal("accepted invalid activation", args)
		}
	}
}
