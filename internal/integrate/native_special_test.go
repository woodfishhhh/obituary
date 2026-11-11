package integrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeSpecialInstallationBoundaries(t *testing.T) {
	for _, agent := range []string{"trae", "trae-cn", "antigravity", "vibe", "kimi"} {
		t.Run(agent, func(t *testing.T) {
			registered, ok := LookupAgent(agent)
			if !ok || registered.PlanMCP == nil {
				t.Fatal("missing agent registration")
			}
			root := t.TempDir()
			t.Setenv("OBIT_USER_HOME", root)
			t.Setenv("VIBE_HOME", filepath.Join(root, ".vibe"))
			t.Setenv("KIMI_SHARE_DIR", filepath.Join(root, ".kimi"))
			s := Scope{Project: t.TempDir()}
			if agent == "kimi" {
				s.Global = true
			}
			commands := Commands{Native: "C:/tools/obit.exe hook " + agent}
			plan, err := registered.Plan(s, commands, false)
			if err != nil || len(plan) == 0 {
				t.Fatal(err)
			}
			first := plan[0].Path
			writeIntegrationFile(t, first, "concurrent")
			if Apply(plan) == nil || read(t, first) != "concurrent" {
				t.Fatal("changed original overwritten")
			}
			s.GuardRM = true
			if _, err := registered.Plan(s, commands, false); err == nil {
				t.Fatal("unsupported delete guard reported installed")
			}
			if registered.PlanSkill == nil {
				t.Fatal("verified skill planner missing")
			}
		})
	}
}

func TestNativeSpecialEmptyAndBOMConfigs(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, agent := range []string{"trae", "trae-cn", "vibe", "kimi"} {
		for _, input := range []string{"", "\xef\xbb\xbf"} {
			t.Run(agent+"/"+map[bool]string{true: "bom", false: "empty"}[input != ""], func(t *testing.T) {
				s := Scope{Project: t.TempDir(), Global: agent == "kimi"}
				root := t.TempDir()
				t.Setenv("KIMI_SHARE_DIR", root)
				path := filepath.Join(s.Project, ".trae", "hooks.json")
				switch agent {
				case "vibe":
					path = filepath.Join(s.Project, ".vibe", "hooks.toml")
				case "kimi":
					path = filepath.Join(root, "config.toml")
				}
				writeIntegrationFile(t, path, input)
				commands := Commands{Native: "C:/tools/obit.exe hook " + agent}
				plan, err := PlanAgent(agent, s, commands, false)
				if err != nil || read(t, path) != input {
					t.Fatal("empty dry-run", err)
				}
				applyIntegration(t, plan, nil)
				if input != "" && !strings.HasPrefix(read(t, path), input) {
					t.Fatal("BOM lost")
				}
				plan, err = PlanAgent(agent, s, commands, false)
				if err != nil || Pending(plan) {
					t.Fatal("empty-derived install not idempotent", err)
				}
				plan, err = PlanAgent(agent, s, Commands{}, true)
				applyIntegration(t, plan, err)
			})
		}
	}
}

func TestNativeSpecialGlobalLinksAndProjectBackups(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, agent := range []string{"trae", "trae-cn", "antigravity", "vibe", "kimi"} {
		t.Run(agent, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("OBIT_USER_HOME", home)
			root := filepath.Join(home, "."+agent)
			if agent == "antigravity" {
				root = filepath.Join(home, ".gemini")
			}
			t.Setenv("VIBE_HOME", root)
			t.Setenv("KIMI_SHARE_DIR", root)
			makeIntegrationJunction(t, root, t.TempDir())
			if _, err := PlanAgent(agent, Scope{Global: true}, Commands{Native: "C:/tools/obit.exe hook " + agent}, false); err == nil {
				t.Fatal("global junction accepted")
			}
		})
	}
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "AGENTS.md")
	writeIntegrationFile(t, path, "user rules\n")
	backup := testBackupPath(t, path)
	if err := os.MkdirAll(filepath.Dir(backup), 0700); err != nil {
		t.Fatal(err)
	}
	makeIntegrationJunction(t, backup, t.TempDir())
	plan, err := PlanKimi(s, Commands{}, false)
	if err == nil {
		err = Apply(plan)
	}
	if err == nil {
		t.Fatal("external backup accepted")
	}
}

func TestNativeSpecialBlockRejectsAmbiguityAndPreservesBOM(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	for _, input := range []string{"\xff", blockEnd, blockBegin, blockBegin + blockEnd + blockBegin + blockEnd, blockEnd + blockBegin} {
		s := Scope{Project: t.TempDir()}
		path := filepath.Join(s.Project, "AGENTS.md")
		writeIntegrationFile(t, path, input)
		if _, err := nativeSpecialBlock(s, path, Awareness, false); err == nil {
			t.Fatal("ambiguous markers accepted", input)
		}
		if read(t, path) != input {
			t.Fatal("invalid file changed")
		}
	}
	for _, input := range []string{"", "\xef\xbb\xbf", "\xef\xbb\xbf# user\r\nkeep\r\n"} {
		s := Scope{Project: t.TempDir()}
		path := filepath.Join(s.Project, "AGENTS.md")
		writeIntegrationFile(t, path, input)
		ch, err := nativeSpecialBlock(s, path, Awareness, false)
		applyIntegration(t, []Change{ch}, err)
		if strings.HasPrefix(input, "\xef\xbb\xbf") && !strings.HasPrefix(read(t, path), "\xef\xbb\xbf") {
			t.Fatal("BOM lost")
		}
		ch, err = nativeSpecialBlock(s, path, Awareness, true)
		applyIntegration(t, []Change{ch}, err)
		if input != "" && read(t, path) != input {
			t.Fatalf("user bytes changed: %q -> %q", input, read(t, path))
		}
		if input == "" {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("owned-only rule remains")
			}
		}
	}
}
