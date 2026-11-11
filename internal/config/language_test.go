package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/i18n"
)

func TestUILanguageUsesAutoOnUnreadableConfiguration(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OBIT_CONFIG_DIR", root)
	path := filepath.Join(root, "config.toml")
	for _, contents := range []string{"[ui]\nlanguage='en'", "[ui]\nlanguage='zh-CN'", "broken = [", "[ui]\nlanguage='unsupported'"} {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		want := i18n.Resolve(i18n.Auto, i18n.SystemLanguage)
		cfg, err := Parse([]byte(contents), path)
		if err == nil {
			want = i18n.Language(cfg.UI.Language)
		}
		if got := UILanguage(); got != want {
			t.Fatalf("%q: got %s want %s", contents, got, want)
		}
	}
	t.Setenv("OBIT_CONFIG_DIR", path) // A file cannot contain config.toml.
	if got := UILanguage(); got != i18n.Resolve(i18n.Auto, i18n.SystemLanguage) {
		t.Fatalf("unreadable config: %s", got)
	}
}
