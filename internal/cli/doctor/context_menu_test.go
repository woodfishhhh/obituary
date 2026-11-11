//go:build windows

package doctor

import (
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestDoctorContextMenuCurrentAndStaleCommands(t *testing.T) {
	reg := contextmenu.WindowsRegistry{Root: testenv.ClassesKey(t)}
	oldReg := ContextMenuRegistry
	t.Cleanup(func() { ContextMenuRegistry = oldReg })
	ContextMenuRegistry = func() contextmenu.Registry { return reg }
	exe := `C:\First Path\obit.exe`
	executable := cmdutil.Executable(func() (string, error) { return exe, nil })
	if c := diagnoseContextMenu(executable); c.Status != "INFO" || !strings.Contains(c.Detail, "not installed") {
		t.Fatal(c)
	}
	plan, err := contextmenu.Plan(reg, exe, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := contextmenu.Apply(reg, plan); err != nil {
		t.Fatal(err)
	}
	if c := diagnoseContextMenu(executable); c.Status != "PASS" || !strings.Contains(c.Detail, exe) {
		t.Fatal(c)
	}
	exe = `D:\New Path\obit.exe`
	if c := diagnoseContextMenu(executable); c.Status != "WARN" || !strings.Contains(c.Detail, "current executable") {
		t.Fatal(c)
	}
	if err := reg.WriteTree(`Directory\Background\shell\Obituary.Policy`, contextmenu.Tree{"": {"MUIVerb": "mine"}}); err != nil {
		t.Fatal(err)
	}
	if c := diagnoseContextMenu(executable); c.Status != "WARN" || !strings.Contains(c.Detail, "modified") {
		t.Fatal(c)
	}
}
