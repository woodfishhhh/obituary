//go:build linux || darwin

package resurrect

import (
	"encoding/json"
	"errors"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
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
	if err != nil || lifecycle.GraveFailed(rows) {
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
	var got []lifecycle.GraveResult
	if !errors.Is(err, cmdutil.ErrReported) || json.Unmarshal([]byte(out), &got) != nil || len(got) != 1 || got[0].Burial.Grave != grave || !strings.Contains(got[0].Error, "is currently at") {
		t.Fatal(out, err)
	}
	if data, err := os.ReadFile(grave); err != nil || string(data) != "kept" {
		t.Fatalf("grave payload %q: %v", data, err)
	}
}

func buryCLIFile(t *testing.T, file string) string {
	t.Helper()
	clitest.WriteFixture(t, file, "kept")
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := svc.Bury([]string{file}, false, "")
	if err != nil || len(rows) != 1 || lifecycle.GraveFailed(rows) {
		t.Fatal(rows, err)
	}
	return rows[0].Burial.Grave
}

func TestRestoreCLIReportsChangedAncestor(t *testing.T) {
	for _, mode := range []string{"stable-alias", "alias-child", "returned-subtree"} {
		t.Run(mode, func(t *testing.T) {
			clitest.IsolateRunner(t)
			dir, _ := clitest.CommandDir(t)
			old, current, file := renamedAncestorFixture(t, dir, mode)
			grave := buryCLIFile(t, file)
			if err := os.Rename(old, current); err != nil {
				t.Fatal(err)
			}
			if mode != "alias-child" {
				clitest.WriteFixture(t, filepath.Join(old, "sentinel"), "untouched")
				branch := "Mid"
				if mode == "stable-alias" {
					branch = "Alias"
				}
				if err := os.Rename(filepath.Join(current, branch), filepath.Join(old, branch)); err != nil {
					t.Fatal(err)
				}
			}
			_, text, err := clitest.Execute(t, NewCmd(), file)
			if !errors.Is(err, cmdutil.ErrReported) || !strings.Contains(text, "original parent "+old+" is currently at "+current+";") {
				t.Fatal(text, err)
			}
			if data, err := os.ReadFile(grave); err != nil || string(data) != "kept" {
				t.Fatal(string(data), err)
			}
			if _, err := os.Stat(file); !os.IsNotExist(err) {
				t.Fatal("unexpected restored file", err)
			}
		})
	}
}

func renamedAncestorFixture(t *testing.T, dir, mode string) (old, current, file string) {
	t.Helper()
	alias, real := filepath.Join(dir, "Alias"), filepath.Join(dir, "Real")
	old, current = filepath.Join(dir, "Old"), filepath.Join(dir, "New")
	if mode == "returned-subtree" {
		return old, current, filepath.Join(old, "Mid", "Leaf", "item")
	}
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	if mode == "stable-alias" {
		if err := os.Mkdir(old, 0700); err != nil {
			t.Fatal(err)
		}
		alias = filepath.Join(old, "Alias")
	}
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}
	if mode == "alias-child" {
		old, current = filepath.Join(alias, "Old"), filepath.Join(alias, "New")
		return old, current, filepath.Join(old, "item")
	}
	return old, current, filepath.Join(alias, "Deep", "Leaf", "item")
}

func TestRestoreCLILinkResolutionReasons(t *testing.T) {
	for _, mode := range []string{"file", "loop"} {
		t.Run(mode, func(t *testing.T) {
			clitest.IsolateRunner(t)
			dir, _ := clitest.CommandDir(t)
			old := filepath.Join(dir, "Old")
			file := filepath.Join(old, "item")
			grave := buryCLIFile(t, file)
			if err := os.Rename(old, old+"-original"); err != nil {
				t.Fatal(err)
			}
			target := old
			en, zh := "links form a loop", "链接形成循环"
			if mode == "file" {
				target = filepath.Join(dir, "regular")
				clitest.WriteFixture(t, target, "sentinel")
				en, zh = "target is not a directory", "目标不是目录"
			}
			if err := os.Symlink(target, old); err != nil {
				t.Fatal(err)
			}
			for _, lang := range []string{"en", "zh-CN"} {
				t.Setenv("OBIT_LANG", lang)
				_, text, err := clitest.Execute(t, NewCmd(), file)
				if !errors.Is(err, cmdutil.ErrReported) || !strings.Contains(text, old) {
					t.Fatal(text, err)
				}
				if lang == "en" && (!strings.Contains(text, en) || !strings.Contains(text, "graveyard")) || lang == "zh-CN" && (!strings.Contains(text, zh) || !strings.Contains(text, "原物仍保留在墓地") || strings.Contains(text, "not a directory") || strings.Contains(text, "too many links")) {
					t.Fatal(text)
				}
			}
			if data, err := os.ReadFile(grave); err != nil || string(data) != "kept" {
				t.Fatal(string(data), err)
			}
		})
	}
}
