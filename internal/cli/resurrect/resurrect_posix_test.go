//go:build linux || darwin

package resurrect

import (
	"encoding/json"
	"errors"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenamedParentCLIReceipts(t *testing.T) {
	clitest.IsolateRunner(t)
	dir, _ := clitest.CommandDir(t)
	old := filepath.Join(dir, "alpha-src")
	current := filepath.Join(dir, "omega-dst")
	file := filepath.Join(old, "payload")
	clitest.WriteFixture(t, file, "kept")
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := svc.Bury([]string{file}, false, "")
	if err != nil || core.GraveFailed(rows) {
		t.Fatal(rows, err)
	}
	grave := rows[0].Burial.Grave
	if err := os.Rename(old, current); err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"en", "zh-CN"} {
		t.Setenv("OBIT_LANG", lang)
		_, text, err := clitest.Execute(t, NewCmd(), file)
		if !errors.Is(err, cmdutil.ErrReported) || !strings.Contains(text, old) || !strings.Contains(text, current) || strings.Contains(text, "unsafe to recycle") {
			t.Fatal(lang, text, err)
		}
		if lang == "en" && !strings.Contains(text, "is currently at") || lang == "zh-CN" && (!strings.Contains(text, "原物仍保留在墓地") || strings.Contains(text, "original parent")) {
			t.Fatal(lang, text)
		}
	}
	out, _, err := clitest.Execute(t, NewCmd(), "--json", file)
	var got []core.GraveResult
	if !errors.Is(err, cmdutil.ErrReported) || json.Unmarshal([]byte(out), &got) != nil || len(got) != 1 || got[0].Burial.Grave != grave || !strings.Contains(got[0].Error, "is currently at") {
		t.Fatal(out, err)
	}
	if data, err := os.ReadFile(grave); err != nil || string(data) != "kept" {
		t.Fatalf("grave payload %q: %v", data, err)
	}
}
