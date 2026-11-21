//go:build windows

package doctor

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestDoctorContextMenuCurrentAndStaleCommands(t *testing.T) {
	t.Setenv("OBIT_LANG", "en")
	english := diagnostics{cmdutil.Localizer(), false}
	reg := contextmenu.WindowsRegistry{Root: testenv.ClassesKey(t)}
	oldReg := ContextMenuRegistry
	t.Cleanup(func() { ContextMenuRegistry = oldReg })
	ContextMenuRegistry = func() contextmenu.Registry { return reg }
	exe := `C:\First Path\obit.exe`
	executable := cmdutil.Executable(func() (string, error) { return exe, nil })
	if c := english.diagnoseContextMenu(executable); c.Status != "INFO" || !strings.Contains(c.Detail, "not installed") {
		t.Fatal(c)
	}
	installMenu(t, reg, exe, i18n.Chinese)
	if c := english.diagnoseContextMenu(executable); c.Status != "INFO" || c.Installation != "label-language-mismatch" {
		t.Fatal(c)
	}
	plan, err := contextmenu.Plan(reg, exe, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := contextmenu.Apply(reg, plan); err != nil {
		t.Fatal(err)
	}
	if c := english.diagnoseContextMenu(executable); c.Status != "PASS" || !strings.Contains(c.Detail, exe) {
		t.Fatal(c)
	}
	for _, path := range []string{`*\shell\Obituary.Mark`, `*\shell\Obituary.Bury`, `Directory\shell\Obituary.Mark`, `Directory\shell\Obituary.Bury`, `Obituary.ContextMenu [shell\01]`, `Obituary.ContextMenu [shell\02]`, `Obituary.ContextMenu [shell\03]`} {
		if c := english.diagnoseContextMenu(executable); !strings.Contains(c.Detail, "PASS "+path) {
			t.Fatal("missing individual menu status", path, c.Detail)
		}
	}
	if err := reg.WriteTree(`Obituary.ContextMenu\shell\02\DropTarget`, contextmenu.Tree{"": {"CLSID": "wrong handler"}}); err != nil {
		t.Fatal(err)
	}
	if c := english.diagnoseContextMenu(executable); c.Status != "WARN" || !strings.Contains(c.Detail, `WARN Obituary.ContextMenu [shell\02]`) || !strings.Contains(c.Detail, `PASS Obituary.ContextMenu [shell\01]`) {
		t.Fatal("doctor did not distinguish broken submenu", c)
	}
	exe = `D:\New Path\obit.exe`
	if c := english.diagnoseContextMenu(executable); c.Status != "WARN" || !strings.Contains(c.Detail, "current executable") {
		t.Fatal(c)
	}
	if err := reg.WriteTree(`Directory\Background\shell\Obituary.Policy`, contextmenu.Tree{"": {"MUIVerb": "mine"}}); err != nil {
		t.Fatal(err)
	}
	if c := english.diagnoseContextMenu(executable); c.Status != "WARN" || !strings.Contains(c.Detail, "modified") {
		t.Fatal(c)
	}
}

func installMenu(t *testing.T, reg contextmenu.Registry, exe string, language i18n.Language) {
	t.Helper()
	changes, err := contextmenu.PlanLanguage(reg, exe, false, language)
	if err != nil {
		t.Fatal(err)
	}
	if err := contextmenu.Apply(reg, changes); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorContextMenuLabelLanguage(t *testing.T) {
	const exe = `C:\First Path\obit.exe`
	for _, tc := range []struct {
		name           string
		labels, ui     i18n.Language
		change, status string
	}{
		{"Chinese labels", i18n.Chinese, i18n.English, "", "INFO"},
		{"English labels", i18n.English, i18n.Chinese, "", "INFO"},
		{"mixed labels", i18n.Chinese, i18n.English, "mixed", "WARN"},
		{"unknown label", i18n.Chinese, i18n.English, "unknown", "WARN"},
		{"different executable", i18n.Chinese, i18n.English, "exe", "WARN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("OBIT_LANG", string(tc.ui))
			reg := clitest.NewMenuRegistry()
			original := ContextMenuRegistry
			t.Cleanup(func() { ContextMenuRegistry = original })
			ContextMenuRegistry = func() contextmenu.Registry { return reg }
			installMenu(t, reg, exe, tc.labels)
			current := exe
			switch tc.change {
			case "mixed", "unknown":
				changes, err := contextmenu.PlanLanguage(reg, exe, false, tc.ui)
				if err != nil {
					t.Fatal(err)
				}
				if tc.change == "unknown" {
					changes[0].After[""]["MUIVerb"] = "Custom label"
				}
				if err := contextmenu.Apply(reg, changes[:1]); err != nil {
					t.Fatal(err)
				}
			case "exe":
				current = `D:\Moved\obit.exe`
			}
			for _, machine := range []bool{false, true} {
				lang := diagnostics{cmdutil.OutputLocalizer(machine), machine}
				c := lang.diagnoseContextMenu(func() (string, error) { return current, nil })
				if machine {
					b, err := json.Marshal(c)
					if err != nil {
						t.Fatal(err)
					}
					t.Logf("JSON: %s", b)
				} else {
					t.Logf("%s %s %s", lang.Label(c.Status), c.Name, c.Detail)
				}
				if c.Status != tc.status {
					t.Errorf("status = %s, want %s", c.Status, tc.status)
				}
				if tc.status == "INFO" {
					note := "Menu labels are in Chinese; the interface is in English."
					if tc.ui == i18n.Chinese {
						note = "右键菜单的标签是英文，当前界面是中文"
						if machine {
							note = "Menu labels are in English; the interface is in Chinese."
						}
					}
					row := lang.Label("PASS") + ` *\shell\Obituary.Mark [] ` + tc.labels.Text(i18n.Mark)
					if c.Installation != "label-language-mismatch" || !strings.Contains(c.Detail, note) || !strings.Contains(c.Detail, row) || !strings.Contains(c.Detail, "obit init --context-menu --yes") || strings.Contains(c.Detail, lang.Label("WARN")) {
						t.Errorf("unexpected language diagnosis: %+v", c)
					}
				} else if c.Installation != "" {
					t.Errorf("unexpected installation: %s", c.Installation)
				}
			}
		})
	}
}
