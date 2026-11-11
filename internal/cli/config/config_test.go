package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
)

func TestConfigShowsWatchStableWindow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OBIT_CONFIG_DIR", dir)
	for _, value := range []string{"10m", "25m"} {
		clitest.WriteFixture(t, filepath.Join(dir, "config.toml"), "[watch]\nstable_window='"+value+"'\n")
		out, _, err := clitest.Execute(t, NewCmd())
		if err != nil || !strings.Contains(out, "[watch]\nstable_window = \""+value+"\"") {
			t.Fatalf("config omitted watch stable window: %s %v", out, err)
		}
	}
}

func TestConfigShowsAgentShells(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OBIT_CONFIG_DIR", dir)
	clitest.WriteFixture(t, filepath.Join(dir, "config.toml"), "codex_windows_shell = 'bash'\n[agent_shells]\nminimax-code = 'powershell'\ncodebuddy = 'bash'\ncodex = 'bash'\n")
	out, _, err := clitest.Execute(t, NewCmd())
	want := "[agent_shells]\n\"codebuddy\" = \"bash\"\n\"codex\" = \"bash\"\n\"minimax-code\" = \"powershell\"\n"
	if err != nil || !strings.Contains(out, want) || !strings.Contains(out, `codex_windows_shell = "bash"`) {
		t.Fatalf("config declarations: %s %v", out, err)
	}
	clitest.WriteFixture(t, filepath.Join(dir, "config.toml"), "codex_windows_shell = 'bash'\n[agent_shells]\ncodex = 'powershell'\n")
	if _, _, err := clitest.Execute(t, NewCmd()); err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("config conflict was hidden: %v", err)
	}
}

func TestConfigShowsCodexWindowsShell(t *testing.T) {
	for _, value := range []string{"", "bash", "powershell"} {
		t.Run(value, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("OBIT_CONFIG_DIR", dir)
			clitest.WriteFixture(t, filepath.Join(dir, "config.toml"), "codex_windows_shell = '"+value+"'\n")
			out, _, err := clitest.Execute(t, NewCmd())
			want := "codex_windows_shell = \"" + value + "\""
			if value == "" {
				want = "# codex_windows_shell is unset"
			}
			if err != nil || !strings.Contains(out, want) || strings.Index(out, want) > strings.Index(out, "[graveyard]") {
				t.Fatalf("config declaration: %s %v", out, err)
			}
		})
	}
}

func TestConfigCreateRejectsChangedExtendedDirectory(t *testing.T) {
	neighbor := filepath.Join(t.TempDir(), "config")
	if err := os.Mkdir(neighbor, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OBIT_CONFIG_DIR", `\\?\`+neighbor+".")
	out, _, err := clitest.Execute(t, NewCmd(), "--create")
	if err == nil {
		t.Fatalf("config create accepted changed directory: %s", out)
	}
	if _, err := os.Stat(filepath.Join(neighbor, "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("config create wrote neighboring directory: %v", err)
	}
}
