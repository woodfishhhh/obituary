package config

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestUIIconDefaultsInTemplate(t *testing.T) {
	for _, key := range []string{"tray = true", "taskbar = true"} {
		if !strings.Contains(Template, key) {
			t.Errorf("missing UI default %s", key)
		}
	}
}

func TestUpdateCheckConfig(t *testing.T) {
	for _, text := range []string{"", Template, "[update]\ncheck=true\n"} {
		cfg, err := Parse([]byte(text), "fixture.toml")
		if err != nil || !cfg.Update.Check {
			t.Fatalf("default: %+v %v", cfg.Update, err)
		}
	}
	cfg, err := Parse([]byte("[update]\ncheck=false\n"), "fixture.toml")
	if err != nil || cfg.Update.Check {
		t.Fatalf("disabled: %+v %v", cfg.Update, err)
	}
	if _, err := Parse([]byte("[update]\ncheck='false'\n"), "fixture.toml"); err == nil {
		t.Fatal("non-boolean accepted")
	}
}

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

func TestUIIconConfigDefaultsAndSwitches(t *testing.T) {
	for _, input := range []string{"", Template, "[ui]\n"} {
		cfg, err := Parse([]byte(input), "fixture")
		if err != nil || !cfg.UI.Tray || !cfg.UI.Taskbar {
			t.Fatal(cfg.UI, err)
		}
	}
	cfg, err := Parse([]byte("[ui]\ntray=false\ntaskbar=false\n"), "fixture")
	if err != nil || cfg.UI.Tray || cfg.UI.Taskbar {
		t.Fatal(cfg.UI, err)
	}
	for _, key := range []string{"tray", "taskbar"} {
		if _, err := Parse([]byte("[ui]\n"+key+"='false'"), "fixture"); err == nil {
			t.Fatal("accepted nonboolean", key)
		}
	}
}

func TestUnknownKeysAreRejected(t *testing.T) {
	for _, tc := range []struct{ text, want string }{
		{"[graveyard]\nretention = 30\n", `fixture.toml:2: unknown key "graveyard.retention" (did you mean "graveyard.retention_days"?)`},
		{"[graveyard]\nretention_day = 30\n", `"graveyard.retention_day" (did you mean "graveyard.retention_days"?)`},
		{"[graveyrd]\nretention_days = 30\n", `fixture.toml:1: unknown key "graveyrd" (did you mean "graveyard"?)`},
		{"[ui]\ntray = false\nlanguag = 'en'\n", `fixture.toml:3: unknown key "ui.languag" (did you mean "ui.language"?)`},
		{"codex_shell = 'bash'\n", `unknown key "codex_shell"`},
		{"[graveyard]\nzzz = 1\n", `unknown key "graveyard.zzz"`},
	} {
		cfg, err := Parse([]byte(tc.text), "fixture.toml")
		if err == nil || !strings.Contains(err.Error(), tc.want) || cfg.Graveyard.RetentionDays != 7 {
			t.Errorf("%q: %+v %v, want %s", tc.text, cfg.Graveyard, err, tc.want)
		}
	}
	if _, err := Parse([]byte("[graveyard]\nzzz = 1\n"), "fixture.toml"); err != nil && strings.Contains(err.Error(), "did you mean") {
		t.Errorf("unrelated key got a suggestion: %v", err)
	}
	_, err := Parse([]byte("[graveyard]\nretention = 30\n[ui]\ntray2 = false\n"), "fixture.toml")
	if err == nil || !strings.Contains(err.Error(), `"graveyard.retention"`) || !strings.Contains(err.Error(), `"ui.tray2"`) {
		t.Errorf("every unknown key is not reported: %v", err)
	}
}

// Every key the template documents or Obituary writes stays accepted, along
// with the case-insensitive spellings go-toml has always matched.
func TestEveryKnownKeyIsAccepted(t *testing.T) {
	text := `codex_windows_shell = "powershell"
[graveyard]
Mode = "recycle"
retention_days = 30
[agent_shells]
codex = "powershell"
some-future-agent = "bash"
[hooks]
session_summary = true
[update]
check = false
[watch]
stable_window = "5m"
[explorer]
enabled = false
everywhere = true
in_git = true
[ui]
language = "en"
tray = false
taskbar = false
`
	cfg, err := Parse([]byte(text), "fixture.toml")
	if err != nil || cfg.Graveyard.Mode != ModeRecycle || cfg.Graveyard.RetentionDays != 30 || cfg.AgentShells["some-future-agent"] != "bash" || cfg.UI.Taskbar {
		t.Fatalf("%+v %v", cfg, err)
	}
	uncommented := strings.NewReplacer("# codex", "codex", "# codebuddy", "codebuddy", "# minimax", "minimax").Replace(Template)
	if _, err := Parse([]byte(uncommented), "fixture.toml"); err != nil {
		t.Fatalf("template with its examples enabled: %v", err)
	}
}

func TestProposalsModeConfig(t *testing.T) {
	for _, tc := range []struct{ text, want string }{
		{"", ProposalsTiered}, {Template, ProposalsTiered}, {"[proposals]\n", ProposalsTiered},
		{"[proposals]\nmode='auto'", ProposalsAuto}, {"[proposals]\nmode='ask'", ProposalsAsk},
		{"[proposals]\nmode='tiered'", ProposalsTiered},
	} {
		cfg, err := Parse([]byte(tc.text), "fixture.toml")
		if err != nil || cfg.Proposals.Mode != tc.want {
			t.Fatalf("%q: %+v %v, want %s", tc.text, cfg.Proposals, err, tc.want)
		}
	}
	for _, text := range []string{"[proposals]\nmode='Auto'", "[proposals]\nmode=''", "[proposals]\nmode=1", "[proposals]\nmodes='auto'", "[proposal]\nmode='auto'"} {
		if _, err := Parse([]byte(text), "fixture.toml"); err == nil {
			t.Fatalf("invalid proposals config accepted: %q", text)
		}
	}
}

func TestSessionCleanupConfig(t *testing.T) {
	for _, text := range []string{"", Template, "[hooks]\nsession_summary=true\n"} {
		cfg, err := Parse([]byte(text), "fixture.toml")
		if err != nil || !cfg.Hooks.SessionCleanup {
			t.Fatalf("default: %+v %v", cfg.Hooks, err)
		}
	}
	cfg, err := Parse([]byte("[hooks]\nsession_cleanup=false\n"), "fixture.toml")
	if err != nil || cfg.Hooks.SessionCleanup {
		t.Fatalf("disabled: %+v %v", cfg.Hooks, err)
	}
}
