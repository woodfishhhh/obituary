package config

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestWatchStableWindowConfig(t *testing.T) {
	for _, tc := range []struct {
		text string
		want time.Duration
	}{
		{"", 10 * time.Minute}, {Template, 10 * time.Minute},
		{"[watch]\nstable_window='25m'", 25 * time.Minute},
		{"[watch]\nstable_window='0s'", 0}, {"[watch]\nstable_window='-1m'", 0},
		{"[watch]\nstable_window='garbage'", 0}, {"[watch]\nstable_window=5", 0},
		{"[watch]\nstable_window='999999999999999999h'", 0},
	} {
		cfg, err := Parse([]byte(tc.text), "fixture.toml")
		if tc.want == 0 {
			if err == nil {
				t.Fatalf("invalid stable window accepted: %s", tc.text)
			}
		} else if err != nil || cfg.Watch.Stability() != tc.want {
			t.Fatalf("watch stable window: %+v %v, want %s", cfg.Watch, err, tc.want)
		}
	}
}

func TestUILanguageConfig(t *testing.T) {
	for _, text := range []string{"", Template, "[ui]\n"} {
		cfg, err := Parse([]byte(text), "fixture.toml")
		if err != nil || cfg.UI.Language != "auto" {
			t.Fatalf("default: %+v %v", cfg.UI, err)
		}
	}
	for _, value := range []string{"auto", "zh-CN", "en", "", "zh", "en-US", "AUTO"} {
		cfg, err := Parse(fmt.Appendf(nil, "[ui]\nlanguage = %q\n", value), "fixture.toml")
		valid := value == "auto" || value == "zh-CN" || value == "en"
		if (err == nil) != valid || valid && cfg.UI.Language != value {
			t.Fatalf("language %q: %+v %v", value, cfg.UI, err)
		}
	}
	if _, err := Parse([]byte("[ui]\nlanguage = true\n"), "fixture.toml"); err == nil {
		t.Fatal("non-string language accepted")
	}
}

func TestAgentShellsConfig(t *testing.T) {
	for _, tc := range []struct{ name, text, agent, want, errorKey string }{
		{"unset", "", "codebuddy", "", ""},
		{"bash", "[agent_shells]\ncodebuddy = 'bash'", "codebuddy", "bash", ""},
		{"powershell", "[agent_shells]\nminimax-code = 'powershell'", "minimax-code", "powershell", ""},
		{"separate-ide", "[agent_shells]\ncodebuddy = 'bash'", "codebuddy-ide", "", ""},
		{"empty", "[agent_shells]\ncodebuddy = ''", "", "", "agent_shells.codebuddy"},
		{"invalid", "[agent_shells]\ncodebuddy = 'cmd'", "", "", "agent_shells.codebuddy"},
		{"case-sensitive", "[agent_shells]\ncodebuddy = 'Bash'", "", "", "agent_shells.codebuddy"},
		{"wrong-type", "[agent_shells]\ncodebuddy = true", "", "", "fixture.toml"},
		{"codex-legacy", "codex_windows_shell = 'bash'", "codex", "bash", ""},
		{"codex-table", "[agent_shells]\ncodex = 'powershell'", "codex", "powershell", ""},
		{"codex-both", "codex_windows_shell = 'bash'\n[agent_shells]\ncodex = 'bash'", "codex", "bash", ""},
		{"codex-empty-legacy", "codex_windows_shell = ''\n[agent_shells]\ncodex = 'bash'", "codex", "bash", ""},
		{"codex-conflict", "codex_windows_shell = 'bash'\n[agent_shells]\ncodex = 'powershell'", "", "", "agent_shells.codex conflicts with codex_windows_shell"},
		{"codex-reverse-conflict", "codex_windows_shell = 'powershell'\n[agent_shells]\ncodex = 'bash'", "", "", "agent_shells.codex conflicts with codex_windows_shell"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Parse([]byte(tc.text), "fixture.toml")
			if tc.errorKey != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errorKey) {
					t.Fatalf("config error: %v, want %s", err, tc.errorKey)
				}
				return
			}
			if err != nil || cfg.WindowsShell(tc.agent) != tc.want {
				t.Fatalf("effective declaration: %+v %v, want %q", cfg, err, tc.want)
			}
		})
	}
}

func TestExplorerDefaultsAndSwitches(t *testing.T) {
	for _, tc := range []struct {
		text                     string
		enabled, git, everywhere bool
	}{
		{"", true, false, false}, {Template, true, false, false},
		{"[explorer]\nenabled=false\nin_git=true\n", false, true, false},
		{"[explorer]\neverywhere=true\n", true, false, true},
	} {
		cfg, err := Parse([]byte(tc.text), "fixture.toml")
		if err != nil || cfg.Explorer.Enabled != tc.enabled || cfg.Explorer.InGit != tc.git || cfg.Explorer.Everywhere != tc.everywhere {
			t.Fatalf("config: %+v %v", cfg, err)
		}
	}
}

func TestCodexWindowsShellConfig(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", "bash", "powershell", "cmd", "Bash"} {
		t.Run(value, func(t *testing.T) {
			cfg, err := Parse([]byte("codex_windows_shell = '"+value+"'\n"), "fixture.toml")
			valid := value == "" || value == "bash" || value == "powershell"
			if (err == nil) != valid || valid && cfg.CodexWindowsShell != value {
				t.Fatalf("declaration %q: %+v %v", value, cfg, err)
			}
		})
	}
	cfg, err := Parse([]byte(Template), "template.toml")
	if err != nil || cfg.CodexWindowsShell != "" || len(cfg.AgentShells) != 0 {
		t.Fatalf("default must stay unset: %+v %v", cfg, err)
	}
}
