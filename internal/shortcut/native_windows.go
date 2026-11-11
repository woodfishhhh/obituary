package shortcut

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

type Native struct{}

// WScript cannot save or load non-ANSI names on English Windows. COM only
// sees an ASCII temporary basename; integrate publishes the bytes as Unicode.
func withLink(data []byte, use func(*ole.IDispatch, string) error) error {
	f, err := os.CreateTemp("", "obit-shortcut-*.lnk")
	if err != nil {
		return err
	}
	path := f.Name()
	defer os.Remove(path)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err = ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	var result *ole.OleError
	if err != nil && (!errors.As(err, &result) || result.Code() != 1) {
		return err
	}
	defer ole.CoUninitialize()
	object, err := oleutil.CreateObject("WScript.Shell")
	if err != nil {
		return err
	}
	defer object.Release()
	shell, err := object.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return err
	}
	defer shell.Release()
	link, err := oleutil.CallMethod(shell, "CreateShortcut", path)
	if err != nil {
		return err
	}
	defer link.Clear()
	return use(link.ToIDispatch(), path)
}

func (Native) Encode(target string) (data []byte, err error) {
	err = withLink(nil, func(link *ole.IDispatch, path string) error {
		for key, value := range map[string]string{"TargetPath": target, "WorkingDirectory": filepath.Dir(target), "IconLocation": target + ",0", "Description": "Obituary init shortcut"} {
			result, err := oleutil.PutProperty(link, key, value)
			if err != nil {
				return err
			}
			result.Clear()
		}
		result, err := oleutil.CallMethod(link, "Save")
		if err != nil {
			return err
		}
		result.Clear()
		data, err = os.ReadFile(path)
		return err
	})
	return data, err
}

func (Native) Target(data []byte) (target string, err error) {
	err = withLink(data, func(link *ole.IDispatch, _ string) error {
		value, err := oleutil.GetProperty(link, "TargetPath")
		if err != nil {
			return err
		}
		defer value.Clear()
		target = value.ToString()
		return nil
	})
	return target, err
}
