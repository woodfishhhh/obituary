package integrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/config"
)

func TestPlanUpdateCheckConfigPreservesFormatting(t *testing.T) {
	for _, content := range []string{"# keep\n[update]\ncheck = true # comment\n[ui]\nlanguage='en'\n", "# keep\n[ui]\nlanguage='en'\n", "\xef\xbb\xbf# keep\r\n[update]\r\ncheck=true # comment\r\n"} {
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		ch, err := PlanUpdateCheckConfig(path, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := Apply([]Change{ch}); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := config.Parse(data, path)
		if err != nil || cfg.Update.Check || !strings.Contains(string(data), "# keep") {
			t.Fatalf("%s %v", data, err)
		}
		if strings.HasPrefix(content, "\xef\xbb\xbf") && !strings.HasPrefix(string(data), "\xef\xbb\xbf") {
			t.Fatal("lost BOM")
		}
		backup, err := os.ReadFile(path + ".bak")
		if err != nil || string(backup) != content {
			t.Fatalf("backup: %s %v", backup, err)
		}
	}
}
