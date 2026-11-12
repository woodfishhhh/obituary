package gui

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/config"
)

func TestSettingsDefaultsAndParseFailure(t *testing.T) {
	testenv.WindowsOnly(t, "GUI settings include Windows desktop integration; POSIX ships the CLI")
	a, _ := fixture(t)
	out, err := a.Settings()
	if err != nil || out.Mode != config.ModeGraveyard || out.RetentionDays != 7 || out.ContextMenuInstalled || out.Autostart || out.Paused {
		t.Fatalf("settings: %+v %v", out, err)
	}
	write(t, out.ConfigPath, "broken = [")
	if _, err := a.Settings(); err == nil {
		t.Fatal("malformed config hidden")
	}
}

func TestPlanSettingsPreservesTOMLCommentsAndUnknownKeys(t *testing.T) {
	for _, original := range []string{
		"# keep\nunknown = 'untouched'\n[graveyard] # grave comment\nmode = 'graveyard' # mode comment\nretention_days = 7 # days comment\n[agent_shells]\ncodex = 'bash'\n",
		"\xef\xbb\xbf# keep\r\n[graveyard]\r\n# days comment\r\nretention_days = 7\r\n",
		"# keep\n[graveyard]",
		"# keep\ngraveyard.mode = 'graveyard'\n",
		"# keep\ngraveyard = { mode = 'graveyard' }\n",
		"# keep\ngraveyard = {}\n",
		"# keep\n[agent_shells]\ncodex='bash'\n",
		"# keep\ngraveyard = {\n# inline comment\nmode = 'graveyard',\n}\n",
	} {
		t.Run(original, func(t *testing.T) {
			a, _ := fixture(t)
			path := filepath.Join(a.deps.ConfigDir, "config.toml")
			write(t, path, original)
			plan, err := a.PlanSettings("recycle", 14)
			if err != nil {
				t.Fatal(err)
			}
			unchanged, _ := os.ReadFile(path)
			if string(unchanged) != original {
				t.Fatal("preview wrote configuration")
			}
			if len(plan.Changes) != 1 || !strings.Contains(plan.Changes[0].After, "retention_days=14") {
				t.Fatalf("preview: %+v", plan)
			}
			if _, err := a.ApplyPlan(plan.ID); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Parse(data, path)
			if err != nil || cfg.Graveyard.Mode != "recycle" || cfg.Graveyard.RetentionDays != 14 {
				t.Fatalf("config: %s %v", data, err)
			}
			for _, marker := range []string{"# keep", "# grave comment", "# mode comment", "# days comment", "unknown = 'untouched'", "codex = 'bash'", "# inline comment"} {
				if strings.Contains(original, marker) && !strings.Contains(string(data), marker) {
					t.Fatalf("lost %s: %s", marker, data)
				}
			}
			if strings.HasPrefix(original, "\xef\xbb\xbf") && !strings.HasPrefix(string(data), "\xef\xbb\xbf") {
				t.Fatal("lost BOM")
			}
			if _, err := a.ApplyPlan(plan.ID); err == nil {
				t.Fatal("plan replayed")
			}
		})
	}
}

func TestPlanSettingsCreationValidationAndStalePreview(t *testing.T) {
	testenv.WindowsOnly(t, "GUI settings include Windows desktop integration; POSIX ships the CLI")
	a, _ := fixture(t)
	plan, err := a.PlanSettings("graveyard", 30)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.ApplyPlan(plan.ID); err != nil {
		t.Fatal(err)
	}
	settings, err := a.Settings()
	if err != nil || settings.RetentionDays != 30 {
		t.Fatalf("create: %+v %v", settings, err)
	}
	for _, days := range []int{0, 3651} {
		if _, err := a.PlanSettings("graveyard", days); err == nil {
			t.Fatal("invalid days accepted")
		}
	}
	if _, err := a.PlanSettings("wrong", 7); err == nil {
		t.Fatal("invalid mode accepted")
	}
	plan, err = a.PlanSettings("recycle", 7)
	if err != nil {
		t.Fatal(err)
	}
	write(t, settings.ConfigPath, "# concurrent editor\n[graveyard]\nretention_days=19\n")
	if _, err := a.ApplyPlan(plan.ID); err == nil {
		t.Fatal("overwrote concurrent editor")
	}
	data, _ := os.ReadFile(settings.ConfigPath)
	if !strings.Contains(string(data), "concurrent editor") {
		t.Fatal("lost concurrent edit")
	}
	if _, err := a.ApplyPlan("unknown"); err == nil {
		t.Fatal("unknown plan accepted")
	}
}

func TestPlanSessionSummaryTogglesConfig(t *testing.T) {
	testenv.WindowsOnly(t, "GUI settings include Windows desktop integration; POSIX ships the CLI")
	a, _ := fixture(t)
	path := filepath.Join(a.deps.ConfigDir, "config.toml")
	write(t, path, "# keep\n[graveyard]\nmode = 'recycle'\n")
	for _, enabled := range []bool{true, false} {
		plan, err := a.PlanSessionSummary(enabled)
		if err != nil || len(plan.Changes) != 1 || plan.Changes[0].After != fmt.Sprintf("session_summary=%t", enabled) {
			t.Fatalf("preview: %+v %v", plan, err)
		}
		if _, err := a.ApplyPlan(plan.ID); err != nil {
			t.Fatal(err)
		}
		out, err := a.Settings()
		if err != nil || out.SessionSummary != enabled || out.Mode != "recycle" || a.deps.Service.SessionSummary != enabled {
			t.Fatalf("settings after apply: %+v %v", out, err)
		}
		if data, _ := os.ReadFile(path); !strings.Contains(string(data), "# keep") {
			t.Fatalf("comment lost: %s", data)
		}
		again, err := a.PlanSessionSummary(enabled)
		if err != nil || len(again.Changes) != 0 {
			t.Fatalf("unchanged value previewed a change: %+v %v", again, err)
		}
	}
}

func TestSettingsReportLifetimeClearing(t *testing.T) {
	testenv.WindowsOnly(t, "GUI settings include Windows desktop integration; POSIX ships the CLI")
	a, root := fixture(t)
	out, err := a.Settings()
	if err != nil || out.Lifetime == nil || out.Lifetime.Buried != 0 {
		t.Fatalf("fresh ledger: %+v %v", out.Lifetime, err)
	}
	bury(t, a, filepath.Join(root, "work", "old.log")) // writes "payload", 7 bytes
	out, err = a.Settings()
	if err != nil || out.Lifetime == nil || out.Lifetime.Buried != 1 || out.Lifetime.BuriedBytes != 7 || out.Lifetime.PurgedBytes != 0 {
		t.Fatalf("after one burial: %+v %v", out.Lifetime, err)
	}
}
