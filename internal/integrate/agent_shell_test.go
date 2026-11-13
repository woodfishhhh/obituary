package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/config"
)

func agentShellFixture(t *testing.T) (Scope, string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("OBIT_CONFIG_DIR", filepath.Join(root, "obit"))
	t.Setenv("OBIT_USER_HOME", filepath.Join(root, "user"))
	t.Setenv("CODEBUDDY_CONFIG_DIR", filepath.Join(root, "codebuddy"))
	t.Setenv("CODEBUDDY_CODE_SHELL", "")
	t.Setenv("CODEBUDDY_CODE_SHELL_PREFIX", "")
	t.Setenv("CODEBUDDY_SANDBOX_IMAGE", "")
	t.Setenv("PATH", root)
	return Scope{Project: filepath.Join(root, "project")}, filepath.Join(root, "obit", "config.toml")
}

func TestAgentWindowsShellPlan(t *testing.T) {
	for _, original := range []string{
		"", "# root comment\nmax_auto_items = 7\n[graveyard]\nretention_days = 14\n",
		"\xef\xbb\xbf# keep\r\n[agent_shells] # shells\r\ncodex = 'bash'\r\n[graveyard]\r\nretention_days = 14\r\n",
		"[agent_shells]", "agent_shells = {}\n", "agent_shells = { codex = 'bash' } # keep\n",
		"agent_shells.codex = 'bash'\n", "[agent_shells]\ncodebuddy = 'powershell'\n",
	} {
		t.Run(original, func(t *testing.T) {
			s, path := agentShellFixture(t)
			t.Setenv("CODEBUDDY_CODE_SHELL", "bash")
			if original != "" {
				writeIntegrationFile(t, path, original)
			}
			ch, err := PlanAgentWindowsShell(s, "codebuddy")
			if err != nil {
				t.Fatal(err)
			}
			if data, _ := os.ReadFile(path); string(data) != original {
				t.Fatal("planning changed config")
			}
			applyIntegration(t, []Change{ch}, nil)
			cfg, err := config.Load()
			want := "bash"
			if strings.Contains(original, "codebuddy = 'powershell'") {
				want = "powershell"
			}
			if err != nil || cfg.WindowsShell("codebuddy") != want {
				t.Fatalf("declaration: %+v %v", cfg, err)
			}
			data := []byte(read(t, path))
			if strings.Contains(original, "# keep") && !bytes.Contains(data, []byte("# keep")) {
				t.Fatal("lost comment")
			}
			if bytes.HasPrefix(data, []byte("\xef\xbb\xbf")) != strings.HasPrefix(original, "\xef\xbb\xbf") {
				t.Fatal("BOM changed")
			}
			before, err := config.Parse([]byte(original), path)
			if err != nil {
				t.Fatal(err)
			}
			if before.AgentShells == nil {
				before.AgentShells = map[string]string{}
			}
			before.AgentShells["codebuddy"] = want
			if !reflect.DeepEqual(before, cfg) {
				t.Fatal("unrelated configuration changed")
			}
			if original != "" && ch.Action != "unchanged" && read(t, path+".bak") != original {
				t.Fatal("backup differs")
			}
			again, err := PlanAgentWindowsShell(s, "codebuddy")
			if err != nil || again.Action != "unchanged" {
				t.Fatalf("not idempotent: %+v %v", again, err)
			}
		})
	}
}

func TestAgentWindowsShellInference(t *testing.T) {
	testenv.WindowsOnly(t, "Windows host shell declarations and PowerShell commandWindows registration")
	for _, tc := range []struct {
		name, agent, env, host, want string
		powershell                   bool
	}{
		{name: "undeclared", agent: "codebuddy"},
		{name: "environment", agent: "codebuddy", env: "powershell", want: "powershell"},
		{name: "host-jsonc", agent: "codebuddy", host: "{// comment\n\"env\":{\"CODEBUDDY_CODE_SHELL\":\"bash\"},}", want: "bash"},
		{name: "conflict", agent: "codebuddy", env: "powershell", host: `{"env":{"CODEBUDDY_CODE_SHELL":"bash"}}`},
		{name: "unsupported", agent: "codebuddy", env: "zsh"},
		{name: "sandbox", agent: "codebuddy", env: "bash", host: `{"sandbox":{"enabled":true}}`},
		{name: "invalid-host", agent: "codebuddy", env: "bash", host: "{"},
		{name: "ide-does-not-inherit-cli", agent: "codebuddy-ide", env: "bash"},
		{name: "minimax-native", agent: "minimax-code", powershell: true, want: "powershell"},
		{name: "minimax-no-shell", agent: "minimax-code"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, path := agentShellFixture(t)
			t.Setenv("CODEBUDDY_CODE_SHELL", tc.env)
			if tc.host != "" {
				writeIntegrationFile(t, filepath.Join(s.Project, ".codebuddy", "settings.json"), tc.host)
			}
			if tc.powershell {
				writeIntegrationFile(t, filepath.Join(os.Getenv("PATH"), "pwsh.exe"), "never executed")
			}
			ch, err := PlanAgentWindowsShell(s, tc.agent)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				for _, want := range []string{path, "[agent_shells]", tc.agent, "\"bash\"", "\"powershell\"", "pass through"} {
					if !strings.Contains(ch.Detail, want) {
						t.Fatalf("missing manual instruction %q: %+v", want, ch)
					}
				}
				if ch.Action != "unchanged" || ch.Manual && !ch.Implicit {
					t.Fatalf("advisory would fail installation or write config: %+v", ch)
				}
			}
			applyIntegration(t, []Change{ch}, nil)
			cfg, err := config.Load()
			if err != nil || cfg.WindowsShell(tc.agent) != tc.want {
				t.Fatalf("inference: %+v %v", cfg, err)
			}
		})
	}
}

func TestAgentWindowsShellPlanCompositionAndConflict(t *testing.T) {
	testenv.WindowsOnly(t, "Windows host shell declarations and PowerShell commandWindows registration")
	s, path := agentShellFixture(t)
	t.Setenv("CODEBUDDY_CODE_SHELL", "bash")
	writeIntegrationFile(t, filepath.Join(os.Getenv("PATH"), "powershell.exe"), "never executed")
	writeIntegrationFile(t, path, "# comment\n[agent_shells]\ncodex = 'bash'\n")
	a, err := PlanAgentWindowsShell(s, "codebuddy")
	if err != nil {
		t.Fatal(err)
	}
	b, err := PlanAgentWindowsShell(s, "minimax-code")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := ComposeAgentChanges([]Change{a, b})
	applyIntegration(t, plan, err)
	cfg, err := config.Load()
	if err != nil || cfg.WindowsShell("codebuddy") != "bash" || cfg.WindowsShell("minimax-code") != "powershell" || cfg.WindowsShell("codex") != "bash" {
		t.Fatalf("composition lost declaration: %+v %v", cfg, err)
	}
	writeIntegrationFile(t, path, "# changed by user\n")
	if err := Apply(plan); err == nil || read(t, path) != "# changed by user\n" {
		t.Fatal("overwrote concurrent config change", err)
	}
}
