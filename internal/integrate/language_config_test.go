package integrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/config"
)

func TestLanguageConfigPreservesSource(t *testing.T) {
	for _, original := range []string{
		"# keep\ncodex_windows_shell = 'bash'\n[ui] # table\nlanguage = 'auto' # language\n[graveyard]\nmode = 'recycle'\n",
		"\xef\xbb\xbf# keep\r\n[ui]\r\nlanguage = 'auto' # language\r\n",
		"# keep\n[ui]", "# keep\n", "# keep\nui.language = 'auto'\n",
		"# keep\nui = { language = 'auto', tray = false }\n",
		"# keep\nui = {}\n", "# keep\nui = {\n# inner\ntaskbar = false,\n}\n",
		"# keep\n[ui]\ntray = false\n",
		"# keep\n[UI]\nLANGUAGE='en'\n[ui]\nlanguage='auto'\n",
	} {
		t.Run(original, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			writeIntegrationFile(t, path, original)
			change, err := PlanLanguageConfig(path, "zh-CN")
			if err != nil || read(t, path) != original {
				t.Fatalf("preview changed source: %v", err)
			}
			if err := Apply([]Change{change}); err != nil {
				t.Fatal(err)
			}
			data := read(t, path)
			cfg, err := config.Parse([]byte(data), path)
			if err != nil || cfg.UI.Language != "zh-CN" || read(t, path+".bak") != original {
				t.Fatalf("applied config or backup: %q %v", data, err)
			}
			want := strings.ReplaceAll(strings.ReplaceAll(original, "'auto'", `"zh-CN"`), "'en'", `"zh-CN"`)
			if strings.Contains(original, "language") || strings.Contains(original, "LANGUAGE") {
				if data != want {
					t.Fatalf("unrelated bytes changed: got %q want %q", data, want)
				}
			} else {
				for _, marker := range []string{"# keep", "# inner", "unknown = 'untouched'"} {
					if strings.Contains(original, marker) && !strings.Contains(data, marker) {
						t.Fatalf("lost %q: %s", marker, data)
					}
				}
			}
		})
	}
}

func TestLanguageConfigCreationAndStaleSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	change, err := PlanLanguageConfig(path, "en")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("planning created configuration")
	}
	if err := Apply([]Change{change}); err != nil {
		t.Fatal(err)
	}
	for _, language := range []string{"en", "zh-CN", "auto"} {
		change, err := PlanLanguageConfig(path, language)
		if err != nil {
			t.Fatal(err)
		}
		if err := Apply([]Change{change}); err != nil {
			t.Fatal(err)
		}
		cfg, err := config.Parse([]byte(read(t, path)), path)
		if err != nil || cfg.UI.Language != language {
			t.Fatalf("config: %+v %v", cfg.UI, err)
		}
	}
	change, err = PlanLanguageConfig(path, "en")
	if err != nil {
		t.Fatal(err)
	}
	writeIntegrationFile(t, path, "# concurrent edit\n")
	if err := Apply([]Change{change}); err == nil || read(t, path) != "# concurrent edit\n" {
		t.Fatalf("concurrent edit overwritten: %v", err)
	}
	if _, err := PlanLanguageConfig(path, "fr"); err == nil {
		t.Fatal("invalid language accepted")
	}
	writeIntegrationFile(t, path, "broken = [")
	if _, err := PlanLanguageConfig(path, "en"); err == nil {
		t.Fatal("malformed config accepted")
	}
}

func TestSessionSummaryConfigToggles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	for _, original := range []string{"", "# keep\n[graveyard]\nmode = 'recycle'\n"} {
		if original != "" {
			writeIntegrationFile(t, path, original)
		}
		for _, enabled := range []bool{true, false} {
			change, err := PlanSessionSummaryConfig(path, enabled)
			if err != nil {
				t.Fatal(err)
			}
			if err := Apply([]Change{change}); err != nil {
				t.Fatal(err)
			}
			data := read(t, path)
			cfg, err := config.Parse([]byte(data), path)
			if err != nil || cfg.Hooks.SessionSummary != enabled {
				t.Fatalf("session_summary=%t not applied: %q %v", enabled, data, err)
			}
			if original != "" && (!strings.Contains(data, "# keep") || cfg.Graveyard.Mode != "recycle") {
				t.Fatalf("unrelated settings lost: %q", data)
			}
		}
	}
}

func TestProposalModeAndCleanupConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	for _, original := range []string{"", "# keep\n[graveyard]\nmode = 'recycle'\n"} {
		if original != "" {
			writeIntegrationFile(t, path, original)
		}
		for _, mode := range []string{config.ProposalsAuto, config.ProposalsAsk, config.ProposalsTiered} {
			change, err := PlanProposalModeConfig(path, mode)
			if err != nil {
				t.Fatal(err)
			}
			if err := Apply([]Change{change}); err != nil {
				t.Fatal(err)
			}
			cleanup, err := PlanSessionCleanupConfig(path, mode != config.ProposalsAsk)
			if err != nil || Apply([]Change{cleanup}) != nil {
				t.Fatal(err)
			}
			data := read(t, path)
			cfg, err := config.Parse([]byte(data), path)
			if err != nil || cfg.Proposals.Mode != mode || cfg.Hooks.SessionCleanup != (mode != config.ProposalsAsk) {
				t.Fatalf("mode=%s not applied: %q %v", mode, data, err)
			}
			if original != "" && (!strings.Contains(data, "# keep") || cfg.Graveyard.Mode != "recycle") {
				t.Fatalf("unrelated settings lost: %q", data)
			}
		}
	}
	if _, err := PlanProposalModeConfig(path, "everything"); err == nil {
		t.Fatal("unknown proposal mode accepted")
	}
}
