package contextmenu

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"golang.org/x/sys/windows"
)

var dropClasses = map[string]string{
	"1d":   "{BF07A7ED-0FE1-468B-A4D7-0E2A145C1001}",
	"7d":   "{BF07A7ED-0FE1-468B-A4D7-0E2A145C1007}",
	"30d":  "{BF07A7ED-0FE1-468B-A4D7-0E2A145C1030}",
	"bury": "{BF07A7ED-0FE1-468B-A4D7-0E2A145C1000}",
}

var (
	ole32            = windows.NewLazySystemDLL("ole32.dll")
	registerClass    = ole32.NewProc("CoRegisterClassObject")
	revokeClass      = ole32.NewProc("CoRevokeClassObject")
	shell32          = windows.NewLazySystemDLL("shell32.dll")
	createShellArray = shell32.NewProc("SHCreateShellItemArrayFromDataObject")
	iidUnknown       = windows.GUID{Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidFactory       = windows.GUID{Data1: 1, Data4: iidUnknown.Data4}
	iidDrop          = windows.GUID{Data1: 0x122, Data4: iidUnknown.Data4}
	iidShellArray    = windows.GUID{Data1: 0xb63ea76d, Data2: 0x1f85, Data3: 0x456f, Data4: [8]byte{0xa1, 0x9c, 0x48, 0x15, 0x9e, 0xfa, 0x85, 0x8b}}
)

// The standard library and installed COM packages do not implement a DropTarget
// local server; this ABI receives one selection without Explorer command limits.
type dropTarget struct {
	vtable    *[7]uintptr
	refs      atomic.Int32
	released  chan struct{}
	once      sync.Once
	selection chan dropSelection
}

type dropSelection struct {
	paths []string
	err   error
}

type dropFactory struct {
	vtable *[5]uintptr
	target *dropTarget
}

const noInterface = 0x80004002

var targetVtable = [7]uintptr{
	syscall.NewCallback(dropQuery), syscall.NewCallback(dropAddRef), syscall.NewCallback(dropRelease),
	dropPointCallback(false), dragOverCallback(), syscall.NewCallback(func(*dropTarget) uintptr { return 0 }), dropPointCallback(true),
}
var factoryVtable = [5]uintptr{
	syscall.NewCallback(factoryQuery), syscall.NewCallback(func(*dropFactory) uintptr { return 2 }),
	syscall.NewCallback(func(*dropFactory) uintptr { return 1 }), syscall.NewCallback(factoryCreate),
	syscall.NewCallback(func(*dropFactory, uint32) uintptr { return 0 }),
}

func dropQuery(d *dropTarget, iid *windows.GUID, out *unsafe.Pointer) uintptr {
	*out = nil
	if *iid != iidUnknown && *iid != iidDrop {
		return noInterface
	}
	*out = unsafe.Pointer(d)
	dropAddRef(d)
	return 0
}

func dropAddRef(d *dropTarget) uintptr { return uintptr(d.refs.Add(1)) }
func dropRelease(d *dropTarget) uintptr {
	n := d.refs.Add(-1)
	if n == 0 {
		d.once.Do(func() { close(d.released) })
	}
	return uintptr(n)
}

func factoryQuery(f *dropFactory, iid *windows.GUID, out *unsafe.Pointer) uintptr {
	*out = nil
	if *iid != iidUnknown && *iid != iidFactory {
		return noInterface
	}
	*out = unsafe.Pointer(f)
	return 0
}

func factoryCreate(f *dropFactory, outer unsafe.Pointer, iid *windows.GUID, out *unsafe.Pointer) uintptr {
	if outer != nil {
		*out = nil
		return 0x80040110 // CLASS_E_NOAGGREGATION
	}
	return dropQuery(f.target, iid, out)
}

// POINTL is passed as one machine word on 64-bit Windows, two on 32-bit Windows.
func dropPointCallback(drop bool) uintptr {
	call := func(d *dropTarget, object *runtimeObject, effect *uint32) uintptr {
		if !drop {
			*effect &= 1 // COPY: Explorer must never remove the source itself.
			return 0
		}
		*effect = 0
		paths, err := selectionPaths(object)
		select {
		case d.selection <- dropSelection{paths, err}:
			return 0
		default:
			return 0x8000ffff // one activation owns one selection
		}
	}
	if runtime.GOARCH == "386" {
		return syscall.NewCallback(func(d *dropTarget, o *runtimeObject, _, _, _ uintptr, effect *uint32) uintptr {
			return call(d, o, effect)
		})
	}
	return syscall.NewCallback(func(d *dropTarget, o *runtimeObject, _, _ uintptr, effect *uint32) uintptr { return call(d, o, effect) })
}

func dragOverCallback() uintptr {
	if runtime.GOARCH == "386" {
		return syscall.NewCallback(func(_ *dropTarget, _, _, _ uintptr, effect *uint32) uintptr { *effect &= 1; return 0 })
	}
	return syscall.NewCallback(func(_ *dropTarget, _, _ uintptr, effect *uint32) uintptr { *effect &= 1; return 0 })
}

// RunDropServer is a separate obitd mode; it never opens the resident service.
func RunDropServer(args []string, execute func(string, []string) ([]byte, error), notify func(string, string) error) error {
	if len(args) < 2 || dropClasses[args[1]] == "" {
		return errors.New("context menu server requires an executable and action")
	}
	if _, err := menuCommand([]string{args[0], args[1], "selection"}); err != nil {
		return err
	}
	return withWinRT(func() error { return serveSelection(args, execute, notify) })
}

func serveSelection(args []string, execute func(string, []string) ([]byte, error), notify func(string, string) error) error {
	clsid, err := windows.GUIDFromString(dropClasses[args[1]])
	if err != nil {
		return err
	}
	d := &dropTarget{vtable: &targetVtable, released: make(chan struct{}), selection: make(chan dropSelection, 1)}
	factory := &dropFactory{vtable: &factoryVtable, target: d}
	defer runtime.KeepAlive(factory)
	var cookie uint32
	// REGCLS_SINGLEUSE removes the factory from activation after the first
	// client, so concurrent menu invocations cannot steal each other's results.
	hr, _, _ := registerClass.Call(uintptr(unsafe.Pointer(&clsid)), uintptr(unsafe.Pointer(factory)), 4, 0, uintptr(unsafe.Pointer(&cookie)))
	if err := winrtError(hr, "register context menu server"); err != nil {
		return err
	}
	defer revokeClass.Call(uintptr(cookie))
	select {
	case selected := <-d.selection:
		err = selected.err
		if err == nil {
			err = Run(append([]string{args[0], args[1]}, selected.paths...), execute, notify)
		} else {
			err = errors.Join(err, notify(config.UILanguage().Text(i18n.OperationIncomplete), err.Error()))
		}
	case <-time.After(time.Minute):
		return errors.New("explorer did not deliver a selection")
	}
	select {
	case <-d.released:
	case <-time.After(time.Minute):
		return errors.Join(err, errors.New("explorer did not release the selection"))
	}
	return err
}

func selectionPaths(object *runtimeObject) ([]string, error) {
	var array *runtimeObject
	hr, _, _ := createShellArray.Call(uintptr(unsafe.Pointer(object)), uintptr(unsafe.Pointer(&iidShellArray)), uintptr(unsafe.Pointer(&array)))
	if err := winrtError(hr, "read Explorer selection"); err != nil {
		return nil, err
	}
	defer array.release()
	var count uint32
	if err := winrtError(array.call(7, uintptr(unsafe.Pointer(&count))), "count Explorer selection"); err != nil {
		return nil, err
	}
	paths := make([]string, 0, count)
	for index := uint32(0); index < count; index++ {
		path, err := selectionPath(array, index)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	if len(paths) == 0 {
		return nil, errors.New("explorer selection is empty")
	}
	return paths, nil
}

func selectionPath(array *runtimeObject, index uint32) (string, error) {
	var item *runtimeObject
	if err := winrtError(array.call(8, uintptr(index), uintptr(unsafe.Pointer(&item))), "read selected item"); err != nil {
		return "", err
	}
	defer item.release()
	var name *uint16
	if err := winrtError(item.call(5, 0x80058000, uintptr(unsafe.Pointer(&name))), "read selected path"); err != nil {
		return "", err
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(name))
	return windows.UTF16PtrToString(name), nil
}
