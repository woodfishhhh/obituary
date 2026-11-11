package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func diagnoseShortcut(executable string, target func(string) (string, error)) DoctorCheck {
	c := DoctorCheck{Name: "start-menu", Status: "WARN"}
	programs, err := userdirs.Programs()
	if err != nil {
		c.Status, c.Detail = "FAIL", err.Error()
		return c
	}
	// install.ps1 uses the bilingual name; Scoop creates an ASCII one
	// because its shortcut code cannot save names outside the ANSI code page.
	var paths []string
	for _, directory := range []string{programs, filepath.Join(programs, "Scoop Apps")} {
		paths = append(paths, filepath.Join(directory, "讣告 Obituary.lnk"), filepath.Join(directory, "Obituary.lnk"))
	}
	for _, path := range paths {
		got, err := target(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			c.Status, c.Detail = "FAIL", err.Error()
			return c
		}
		want := filepath.Join(filepath.Dir(executable), "obit-gui.exe")
		if !strings.EqualFold(filepath.Clean(got), filepath.Clean(want)) {
			c.Detail = "Start menu shortcut targets a different installation: " + got
			return c
		}
		c.Status, c.Detail = "PASS", path+" -> "+got
		return c
	}
	c.Detail = "Start menu shortcut is absent; the PowerShell installer and Scoop create it on amd64"
	return c
}

// Reuse the installed go-ole wrapper for Windows' standard WScript.Shell API;
// no custom Shell Link binary parser and no GUI process are needed.
func shortcutTarget(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	// WScript.Shell cannot load a .lnk whose name has characters outside the
	// system ANSI code page (the Chinese name on English Windows); read an
	// ASCII-named private copy instead.
	copyFile, err := os.CreateTemp("", "obit-shortcut-*.lnk")
	if err != nil {
		return "", err
	}
	defer os.Remove(copyFile.Name())
	if _, err := copyFile.Write(data); err != nil {
		copyFile.Close()
		return "", err
	}
	if err := copyFile.Close(); err != nil {
		return "", err
	}
	path = copyFile.Name()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err = ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	var hresult *ole.OleError
	if err != nil && (!errors.As(err, &hresult) || hresult.Code() != 1) {
		return "", err
	}
	defer ole.CoUninitialize()
	object, err := oleutil.CreateObject("WScript.Shell")
	if err != nil {
		return "", err
	}
	defer object.Release()
	shell, err := object.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return "", err
	}
	defer shell.Release()
	link, err := oleutil.CallMethod(shell, "CreateShortcut", path)
	if err != nil {
		return "", err
	}
	defer link.Clear()
	value, err := oleutil.GetProperty(link.ToIDispatch(), "TargetPath")
	if err != nil {
		return "", err
	}
	defer value.Clear()
	return value.ToString(), nil
}
