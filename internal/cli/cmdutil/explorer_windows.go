//go:build windows

package cmdutil

import (
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/spf13/cobra"
	"golang.org/x/sys/windows"
)

// SkipExplorerCheck lets Cobra's double-click check stand only when the parent
// might be Explorer. Cobra's check lists every process twice (about 24 ms per
// call, more under load) to read one parent's name; the parent is one kernel
// query away. Whenever the parent cannot be named, or is Explorer, Cobra's own
// check still decides, so a double-click behaves exactly as before.
func SkipExplorerCheck() {
	if parentIsOtherThanExplorer() {
		cobra.MousetrapHelpText = ""
	}
}

func parentIsOtherThanExplorer() bool {
	var info windows.PROCESS_BASIC_INFORMATION
	if windows.NtQueryInformationProcess(windows.CurrentProcess(), windows.ProcessBasicInformation,
		unsafe.Pointer(&info), uint32(unsafe.Sizeof(info)), nil) != nil || info.InheritedFromUniqueProcessId == 0 {
		return false
	}
	parent, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(info.InheritedFromUniqueProcessId))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(parent)
	var image [windows.MAX_LONG_PATH]uint16
	size := uint32(len(image))
	if windows.QueryFullProcessImageName(parent, 0, &image[0], &size) != nil || size == 0 {
		return false
	}
	return !strings.EqualFold(filepath.Base(windows.UTF16ToString(image[:size])), "explorer.exe")
}
