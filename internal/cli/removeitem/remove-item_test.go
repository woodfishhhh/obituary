//go:build windows

package removeitem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	resurrectcmd "github.com/woodfishhhh/obituary/internal/cli/resurrect"
	"github.com/woodfishhhh/obituary/internal/core"
	"golang.org/x/sys/windows"
)

func TestRemoveItemRejectsMultiplePaths(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, args := range [][]string{
		{"a", "b"}, {"-Path", "a", "b"}, {"-LiteralPath", "a", "b"},
		{"a", "-Force", "b"}, {"-LiteralPath", "a", "-Recurse", "b"},
		{"-Path", "a", "-LiteralPath", "b"}, {"a", "-Path", "b"}, {"a,", "b"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			// Every operand exists and is deletable: no missing-file or Guard
			// rejection can hide an incorrect multiple-path binding.
			for _, file := range []string{"a", "b", "a,"} {
				clitest.WriteFixture(t, filepath.Join(repo, file), file)
			}
			if _, _, err := clitest.Execute(t, itemCommand(), args...); err == nil {
				t.Error("multiple paths accepted")
			}
			for _, file := range []string{"a", "b", "a,"} {
				if data, err := os.ReadFile(filepath.Join(repo, file)); err != nil || string(data) != file {
					t.Errorf("invalid binding moved %s: %q %v", file, data, err)
				}
			}
			st, err := store.Read()
			if err != nil || len(st.Burials) != 0 {
				t.Fatalf("invalid binding buried paths: %+v %v", st.Burials, err)
			}
		})
	}
}

func TestRemoveItemHelpScope(t *testing.T) {
	clitest.CommandDir(t)
	out, stderr, err := clitest.Execute(t, itemCommand(), "--help")
	if err != nil || stderr != "" {
		t.Fatalf("--help: %s %s %v", out, stderr, err)
	}
	for _, text := range []string{"not a complete replacement", "exactly one path", "$WhatIfPreference", "$ConfirmPreference", "native Remove-Item"} {
		if !strings.Contains(out, text) {
			t.Errorf("missing help limitation %q: %s", text, out)
		}
	}
}

func TestRemoveItemUnicodeDash(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, dash := range []string{"–", "—", "―"} {
		for _, parameter := range []string{"", "-Path", "-LiteralPath"} {
			t.Run(dash+parameter, func(t *testing.T) {
				clitest.IsolateRunner(t)
				repo, store := clitest.CommandDir(t)
				name := dash + "WhatIf"
				clitest.WriteFixture(t, filepath.Join(repo, name), "keep")
				args := []string{name}
				if parameter != "" {
					args = append([]string{parameter}, args...)
				}
				_, stderr, err := clitest.Execute(t, itemCommand(), args...)
				if err == nil || !strings.Contains(stderr, "Unicode dash") {
					t.Errorf("Unicode parameter prefix accepted: %s %v", stderr, err)
				}
				if data, err := os.ReadFile(filepath.Join(repo, name)); err != nil || string(data) != "keep" {
					t.Errorf("Unicode parameter-like path buried: %q %v", data, err)
				}
				st, err := store.Read()
				if err != nil || len(st.Burials) != 0 {
					t.Fatalf("Unicode parameter burial: %+v %v", st.Burials, err)
				}
				if _, stderr, err := clitest.Execute(t, itemCommand(), "-LiteralPath", `.\`+name); err != nil {
					t.Fatalf("explicit path refused: %s %v", stderr, err)
				}
				st, err = store.Read()
				if err != nil || len(st.Burials) != 1 {
					t.Fatalf("explicit path burial: %+v %v", st.Burials, err)
				}
			})
		}
	}
}

func TestRemoveItemRejectsDoubleQuotes(t *testing.T) {
	clitest.IsolateRunner(t)
	repo, store := clitest.CommandDir(t)
	clitest.WriteFixture(t, filepath.Join(repo, "keep.txt"), "keep")
	for _, args := range [][]string{{`keep"".txt`}, {"-LiteralPath", `keep"".txt`}, {"-Path", `keep".txt`}} {
		if _, stderr, err := clitest.Execute(t, itemCommand(), args...); err == nil || !strings.Contains(stderr, "double quote") {
			t.Errorf("quote rejection must precede filesystem lookup: %s %v", stderr, err)
		}
	}
	if data, err := os.ReadFile(filepath.Join(repo, "keep.txt")); err != nil || string(data) != "keep" {
		t.Fatalf("stripped quote selected keep.txt: %q %v", data, err)
	}
	st, err := store.Read()
	if err != nil || len(st.Burials) != 0 {
		t.Fatalf("invalid path buried files: %+v %v", st.Burials, err)
	}
}

func TestRemoveItemWildcardRefusalKeepsFiles(t *testing.T) {
	for _, parameter := range []string{"", "-Path"} {
		t.Run(parameter, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			for _, name := range []string{"keep].txt", "a[1].txt", "a1.txt"} {
				clitest.WriteFixture(t, filepath.Join(repo, name), "keep")
			}
			for _, name := range []string{"keep].txt", "a[1].txt"} {
				args := []string{name}
				if parameter != "" {
					args = append([]string{parameter}, args...)
				}
				_, stderr, err := clitest.Execute(t, itemCommand(), args...)
				if err == nil || !strings.Contains(stderr, "wildcards are unsupported") || !strings.Contains(stderr, "obit rm") || !strings.Contains(stderr, "-LiteralPath") {
					t.Errorf("wildcard refusal/guidance missing: %s %v", stderr, err)
				}
			}
			for _, name := range []string{"keep].txt", "a[1].txt", "a1.txt"} {
				if data, err := os.ReadFile(filepath.Join(repo, name)); err != nil || string(data) != "keep" {
					t.Errorf("refused path changed: %s %q %v", name, data, err)
				}
			}
			st, err := store.Read()
			if err != nil || len(st.Burials) != 0 {
				t.Fatalf("wildcard refusal recorded burials: %+v %v", st.Burials, err)
			}
		})
	}
}

func TestRemoveItemLiteralNames(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	clitest.IsolateRunner(t)
	repo, store := clitest.CommandDir(t)
	for _, name := range []string{"a[1].txt", "keep].txt", "a😀.txt"} {
		clitest.WriteFixture(t, filepath.Join(repo, name), name)
		if _, stderr, err := clitest.Execute(t, itemCommand(), "-LiteralPath", name); err != nil {
			t.Fatalf("literal filename refused: %s %v", stderr, err)
		}
	}
	st, err := store.Read()
	if err != nil || len(st.Burials) != 3 {
		t.Fatalf("literal burials: %+v %v", st.Burials, err)
	}
	for _, burial := range st.Burials {
		if _, stderr, err := clitest.Execute(t, resurrectcmd.NewCmd(), fmt.Sprint(burial.ID)); err != nil {
			t.Fatalf("literal resurrection: %s %v", stderr, err)
		}
		if data, err := os.ReadFile(burial.Path); err != nil || string(data) != filepath.Base(burial.Path) {
			t.Fatalf("literal round trip: %q %v", data, err)
		}
	}
}

func TestRemoveItemWildcardHelp(t *testing.T) {
	out, _, err := clitest.Execute(t, itemCommand(), "--help")
	for _, text := range []string{"wildcards", "*", "?", "[", "]", "bash", "obit rm", "-LiteralPath"} {
		if err != nil || !strings.Contains(out, text) {
			t.Errorf("help missing %q: %s %v", text, out, err)
		}
	}
}

func TestRemoveItemLiteralComma(t *testing.T) {
	clitest.IsolateRunner(t)
	repo, store := clitest.CommandDir(t)
	for _, name := range []string{"a", "b", "a,b"} {
		clitest.WriteFixture(t, filepath.Join(repo, name), name)
	}
	if _, stderr, err := clitest.Execute(t, itemCommand(), "-LiteralPath", "a,b"); err != nil {
		t.Fatalf("literal comma: %s %v", stderr, err)
	}
	st, err := store.Read()
	if err != nil || len(st.Burials) != 1 || filepath.Base(st.Burials[0].Path) != "a,b" {
		t.Fatalf("comma split argv: %+v %v", st.Burials, err)
	}
	for _, name := range []string{"a", "b"} {
		if data, err := os.ReadFile(filepath.Join(repo, name)); err != nil || string(data) != name {
			t.Fatalf("unselected file changed: %s %q %v", name, data, err)
		}
	}
}

func TestRemoveItemReadOnlyDirectories(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, tc := range []struct {
		name, target string
		attrs        uint32
	}{
		{"empty", "tree", windows.FILE_ATTRIBUTE_READONLY},
		{"readonly-child", "tree/child", windows.FILE_ATTRIBUTE_READONLY},
		{"hidden-child", "tree/child", windows.FILE_ATTRIBUTE_HIDDEN},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			path := filepath.Join(repo, tc.target)
			if err := os.MkdirAll(path, 0o700); err != nil {
				t.Fatal(err)
			}
			setItemAttrs(t, path, tc.attrs)
			args := []string{"tree"}
			if tc.name != "empty" {
				args = append([]string{"-Recurse"}, args...)
			}
			if _, _, err := clitest.Execute(t, itemCommand(), args...); err == nil {
				t.Fatal("directory needing Force was buried")
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatal("refusal moved directory", err)
			}
			st, err := store.Read()
			if err != nil || len(st.Burials) != 0 {
				t.Fatalf("refusal recorded burial: %+v %v", st.Burials, err)
			}
			if _, stderr, err := clitest.Execute(t, itemCommand(), append([]string{"-Force"}, args...)...); err != nil {
				t.Fatalf("Force: %s %v", stderr, err)
			}
			st, err = store.Read()
			if err != nil || len(st.Burials) != 1 {
				t.Fatalf("forced directory: %+v %v", st.Burials, err)
			}
		})
	}
}

func TestRemoveItemRecurseLiteralAndForce(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{"recursive", []string{"-Recurse", "tree"}, 1},
		{"literal", []string{"-LiteralPath", "a[1].log"}, 1},
		{"forced-tree", []string{"-Recurse", "-Force", "tree"}, 1},
		{"forced-hidden", []string{"-Force", "hidden"}, 1},
		{"forced-readonly", []string{"-Force", "readonly"}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			for _, path := range []string{"tree/nested/file", "a[1].log", "b.log", "hidden", "readonly"} {
				clitest.WriteFixture(t, filepath.Join(repo, path), "keep "+path)
			}
			setItemAttrs(t, filepath.Join(repo, "hidden"), windows.FILE_ATTRIBUTE_HIDDEN)
			setItemAttrs(t, filepath.Join(repo, "readonly"), windows.FILE_ATTRIBUTE_READONLY)
			out, stderr, err := clitest.Execute(t, itemCommand(), tc.args...)
			if err != nil || out+stderr != "" {
				t.Fatalf("remove-item: %s %s %v", out, stderr, err)
			}
			st, err := store.Read()
			if err != nil || len(st.Burials) != tc.want {
				t.Fatalf("burials: %+v %v", st.Burials, err)
			}
			for _, grave := range st.Burials {
				if _, _, err := clitest.Execute(t, resurrectcmd.NewCmd(), fmt.Sprint(grave.ID)); err != nil {
					t.Fatal(err)
				}
			}
			for _, path := range []string{"tree/nested/file", "a[1].log", "b.log", "hidden", "readonly"} {
				if got, err := os.ReadFile(filepath.Join(repo, path)); err != nil || string(got) != "keep "+path {
					t.Fatalf("lost %s: %q %v", path, got, err)
				}
			}
		})
	}
}

func TestRemoveItemParameterValidation(t *testing.T) {
	clitest.IsolateRunner(t)
	repo, store := clitest.CommandDir(t)
	clitest.WriteFixture(t, filepath.Join(repo, "keep"), "keep")
	for _, args := range [][]string{{"keep", "-Confirm"}, {"keep", "-Unknown"}, {"-Path", "keep", "-LiteralPath", "keep"}} {
		if _, _, err := clitest.Execute(t, itemCommand(), args...); err == nil {
			t.Fatalf("unsupported accepted: %v", args)
		}
		if _, err := os.Stat(filepath.Join(repo, "keep")); err != nil {
			t.Fatal("invalid option moved earlier operand", err)
		}
	}
	st, err := store.Read()
	if err != nil || len(st.Burials) != 0 {
		t.Fatalf("invalid parameters buried files: %+v %v", st.Burials, err)
	}
}

func TestRemoveItemKeepsTreeRequiringForce(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	clitest.IsolateRunner(t)
	repo, store := clitest.CommandDir(t)
	path := filepath.Join(repo, "tree", "readonly")
	clitest.WriteFixture(t, path, "irreplaceable")
	setItemAttrs(t, path, windows.FILE_ATTRIBUTE_READONLY)
	if _, _, err := clitest.Execute(t, itemCommand(), "-Recurse", "tree"); err == nil {
		t.Fatal("tree swallowed read-only child")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "irreplaceable" {
		t.Fatalf("child lost: %q %v", got, err)
	}
	if _, _, err := clitest.Execute(t, itemCommand(), "-Recurse", "-Force", "tree"); err != nil {
		t.Fatal(err)
	}
	st, err := store.Read()
	if err != nil || len(st.Burials) != 1 {
		t.Fatalf("forced tree: %+v %v", st.Burials, err)
	}
}

func TestRemoveItemRefusals(t *testing.T) {
	clitest.IsolateRunner(t)
	repo, store := clitest.CommandDir(t)
	for _, path := range []string{"tree/child", "hidden", "readonly", "protected", "ok"} {
		clitest.WriteFixture(t, filepath.Join(repo, path), "irreplaceable")
	}
	setItemAttrs(t, filepath.Join(repo, "hidden"), windows.FILE_ATTRIBUTE_HIDDEN)
	setItemAttrs(t, filepath.Join(repo, "readonly"), windows.FILE_ATTRIBUTE_READONLY)
	svc, err := app.Open()
	if err != nil {
		t.Fatal(err)
	}
	svc.Guard.Keep = append(svc.Guard.Keep, filepath.Join(repo, "protected"))
	old := cmdutil.Open
	cmdutil.Open = func() (*core.Service, error) { return svc, nil }
	t.Cleanup(func() { cmdutil.Open = old })
	for _, path := range []string{"tree", "hidden", "readonly", "protected"} {
		_, stderr, err := clitest.Execute(t, itemCommand(), "-Path", path)
		if !errors.Is(err, cmdutil.ErrReported) || !strings.Contains(stderr, "Remove-Item :") || !strings.Contains(stderr, "CategoryInfo") {
			t.Fatalf("PowerShell error: %s %v", stderr, err)
		}
	}
	if _, stderr, err := clitest.Execute(t, itemCommand(), "-Path", "ok"); err != nil {
		t.Fatalf("ordinary path: %s %v", stderr, err)
	}
	for _, path := range []string{"tree/child", "hidden", "readonly", "protected"} {
		if got, err := os.ReadFile(filepath.Join(repo, path)); err != nil || string(got) != "irreplaceable" {
			t.Errorf("refusal lost %s: %q %v", path, got, err)
		}
	}
	st, err := store.Read()
	if err != nil || len(st.Burials) != 1 || filepath.Base(st.Burials[0].Path) != "ok" {
		t.Fatalf("successful path: %+v %v", st.Burials, err)
	}
	if _, _, err := clitest.Execute(t, itemCommand(), "-Force", "protected"); err == nil {
		t.Fatal("Force bypassed Guard")
	}
	_, stderr, err := clitest.Execute(t, itemCommand(), "-Force", "missing")
	if err == nil || !strings.Contains(stderr, "does not exist") {
		t.Fatalf("Force suppressed missing or Guard: %s %v", stderr, err)
	}
	if _, err := os.Stat(filepath.Join(repo, "protected")); err != nil {
		t.Fatal("Force bypassed Guard", err)
	}
}

func TestRemoveItemRejectsTildeAndWildcardEscapes(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, ref := range []string{"~", "~/keep", "~name", "keep`name", "keep`?"} {
		for _, parameter := range []string{"-Path", "-LiteralPath"} {
			if parameter == "-LiteralPath" && strings.ContainsRune(ref, '`') {
				continue // literal backticks are exercised separately below
			}
			t.Run(parameter+"/"+ref, func(t *testing.T) {
				clitest.IsolateRunner(t)
				repo, store := clitest.CommandDir(t)
				for _, path := range []string{"~/keep", "~name", "keep`name", "keep`x"} {
					clitest.WriteFixture(t, filepath.Join(repo, path), "keep")
				}
				_, stderr, err := clitest.Execute(t, itemCommand(), "-Recurse", "-Force", parameter, ref)
				if err == nil || (!strings.Contains(stderr, "tilde") && !strings.Contains(stderr, "backtick")) {
					t.Errorf("provider ambiguity not rejected: %s %v", stderr, err)
				}
				st, err := store.Read()
				if err != nil || len(st.Burials) != 0 {
					t.Fatalf("ambiguous provider path buried: %+v %v", st.Burials, err)
				}
			})
		}
	}
	t.Run("literal-backtick", func(t *testing.T) {
		clitest.IsolateRunner(t)
		repo, store := clitest.CommandDir(t)
		clitest.WriteFixture(t, filepath.Join(repo, "keep`name"), "literal")
		if _, stderr, err := clitest.Execute(t, itemCommand(), "-LiteralPath", "keep`name"); err != nil {
			t.Fatalf("literal backtick: %s %v", stderr, err)
		}
		st, err := store.Read()
		if err != nil || len(st.Burials) != 1 || filepath.Base(st.Burials[0].Path) != "keep`name" {
			t.Fatalf("literal backtick lost: %+v %v", st.Burials, err)
		}
	})
}
