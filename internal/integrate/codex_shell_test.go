package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/config"
)

func TestCodexWindowsShellPlan(t *testing.T) {
	testenv.WindowsOnly(t, "Windows host shell declarations and PowerShell commandWindows registration")
	for _, tc := range []struct {
		name, original, codex, want string
		noShell, warning            bool
	}{
		{name: "native-default", want: "powershell"},
		{name: "native-config", codex: "[windows]\nsandbox = 'elevated'\n", want: "powershell"},
		{name: "preserve-bash", original: "codex_windows_shell = 'bash'\n", want: "bash"},
		{name: "preserve-agent-table", original: "[agent_shells]\ncodex = 'bash'\n", want: "bash"},
		{name: "preserve-empty", original: "codex_windows_shell = ''\n"},
		{name: "preserve-table-and-bom", original: "\xef\xbb\xbf# user comment\n[graveyard]\nretention_days = 14\n", want: "powershell"},
		{name: "unreadable-config", codex: "[", warning: true},
		{name: "opaque-desktop", codex: "[desktop]\ndefaultShell = 'git-bash'\n", warning: true},
		{name: "no-shell", noShell: true, warning: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("OBIT_CONFIG_DIR", filepath.Join(root, "obit"))
			t.Setenv("CODEX_HOME", filepath.Join(root, "codex"))
			t.Setenv("PATH", root)
			if !tc.noShell {
				writeIntegrationFile(t, filepath.Join(root, "powershell.exe"), "fixture; never executed")
			}
			writeIntegrationFile(t, filepath.Join(root, "codex", "config.toml"), tc.codex)
			path, err := config.Path()
			if err != nil {
				t.Fatal(err)
			}
			if tc.original != "" {
				writeIntegrationFile(t, path, tc.original)
			}
			ch, err := PlanCodexWindowsShell(Scope{Global: true})
			if err != nil {
				t.Fatal(err)
			}
			if tc.warning != strings.Contains(ch.Detail, "set codex_windows_shell") {
				t.Fatalf("manual declaration guidance: %+v", ch)
			}
			if got, _ := os.ReadFile(path); string(got) != tc.original {
				t.Fatal("planning wrote config")
			}
			if err := Apply([]Change{ch}); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load()
			if err != nil || cfg.WindowsShell("codex") != tc.want {
				t.Fatalf("shell plan: config=%+v error=%v plan=%+v", cfg, err, ch)
			}
			if tc.warning {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("uncertain inference wrote config: %v", err)
				}
			}
			if tc.original != "" {
				got, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if strings.HasPrefix(tc.name, "preserve-table") {
					if !bytes.HasPrefix(got, []byte("\xef\xbb\xbf")) || !bytes.Contains(got, []byte(tc.original[3:])) || cfg.Graveyard.RetentionDays != 14 {
						t.Fatalf("user content changed: %q", got)
					}
					if backup, _ := os.ReadFile(path + ".bak"); string(backup) != tc.original {
						t.Fatal("missing original backup")
					}
				} else if string(got) != tc.original {
					t.Fatalf("explicit declaration replaced: %s", got)
				}
			}
			again, err := PlanCodexWindowsShell(Scope{Global: true})
			if err != nil || again.Action != "unchanged" {
				t.Fatalf("not idempotent: %+v %v", again, err)
			}
			if got := read(t, filepath.Join(root, "codex", "config.toml")); got != tc.codex {
				t.Fatal("Codex configuration modified")
			}
		})
	}
}

func TestCodexWindowsShellPlanProjectAndConflict(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OBIT_CONFIG_DIR", filepath.Join(root, "obit"))
	t.Setenv("CODEX_HOME", filepath.Join(root, "codex"))
	t.Setenv("PATH", root)
	writeIntegrationFile(t, filepath.Join(root, "powershell.exe"), "fixture; never executed")
	project := filepath.Join(root, "project")
	writeIntegrationFile(t, filepath.Join(project, ".codex", "config.toml"), "[")
	ch, err := PlanCodexWindowsShell(Scope{Project: project})
	if err != nil || ch.Action != "unchanged" || !strings.Contains(ch.Detail, "set codex_windows_shell") {
		t.Fatalf("project config ignored: %+v %v", ch, err)
	}
	ch, err = PlanCodexWindowsShell(Scope{Global: true})
	if err != nil {
		t.Fatal(err)
	}
	writeIntegrationFile(t, ch.Path, "codex_windows_shell = 'bash'\n")
	if err := Apply([]Change{ch}); err == nil {
		t.Fatal("changed original overwritten")
	}
	if read(t, ch.Path) != "codex_windows_shell = 'bash'\n" {
		t.Fatal("user update lost")
	}
}
