package cmdutil_test

import (
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"strings"
	"testing"
)

func TestRestoreParentErrorLanguage(t *testing.T) {
	for _, lang := range []string{"en", "zh-CN"} {
		t.Run(lang, func(t *testing.T) {
			t.Setenv("OBIT_LANG", lang)
			for _, kind := range []string{"renamed", "unverified", "link", "file", "file_windows", "permission", "missing"} {
				e := &corefs.RestoreParentError{Kind: kind, Path: "/alpha-src", Current: "/omega-dst", Cause: corefs.ErrUnsafe}
				got := cmdutil.ReferenceText(cmdutil.Localizer(), e, e.Error())
				if !strings.Contains(got, e.Path) || kind == "renamed" && !strings.Contains(got, e.Current) || strings.Contains(got, "unsafe to recycle") {
					t.Fatal(kind, got)
				}
				if lang == "en" && got != e.Error() || lang == "zh-CN" && (!strings.Contains(got, "墓地") || strings.Contains(got, "graveyard")) {
					t.Fatal(kind, got)
				}
			}
		})
	}
}
