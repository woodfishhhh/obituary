package update

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/update"
)

func TestUpdateReadsCacheAndChecksOnRequest(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OBIT_HOME", filepath.Join(root, "home"))
	t.Setenv("OBIT_CONFIG_DIR", filepath.Join(root, "config"))
	clitest.WriteFixture(t, filepath.Join(root, "config", "config.toml"), "[update]\ncheck=false\n")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"tag_name":"v9.0.0","html_url":"https://github.com/woodfishhhh/obituary/releases/tag/v9.0.0"}`)
	}))
	defer server.Close()
	checker := &update.Checker{BaseURL: server.URL}
	for _, check := range []bool{false, true, false} {
		cmd := newCmd(checker)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{})
		if check {
			cmd.SetArgs([]string{"--check"})
		}
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "Current:") || !strings.Contains(out.String(), "Automatic checks: false") || !strings.Contains(out.String(), "Channel: other") {
			t.Fatal(out.String())
		}
		if calls > 0 && (!strings.Contains(out.String(), "Latest: 9.0.0") || !strings.Contains(out.String(), "Newer: true") || !strings.Contains(out.String(), "Release: https://")) {
			t.Fatal(out.String())
		}
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestUpdateApplyRejectsUnsupportedInstallation(t *testing.T) {
	t.Setenv("OBIT_HOME", t.TempDir())
	cmd := newCmd(nil)
	cmd.SetArgs([]string{"--apply"})
	// The test binary is a development build in a folder without the release programs.
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "not available") || !strings.Contains(err.Error(), "development build") {
		t.Fatal(err)
	}
}

func TestShowHowToUpdate(t *testing.T) {
	for name, tc := range map[string]struct {
		ap      update.Applicability
		command string
		newer   bool
		want    string
	}{
		"command":            {update.Applicability{Channel: "scoop", Can: true}, "scoop update obituary", true, "Update: scoop update obituary"},
		"manual can":         {update.Applicability{Channel: "other", Can: true}, "", true, "obit update --apply"},
		"manual blocked":     {update.Applicability{Channel: "other", Blocked: update.BlockedFiles, Detail: "obitd.exe"}, "", true, "plain files: obitd.exe"},
		"manual up to date":  {update.Applicability{Channel: "other", Can: true}, "", false, ""},
		"blocked up to date": {update.Applicability{Channel: "other", Blocked: update.BlockedDev}, "", false, ""},
	} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			showHowToUpdate(&out, tc.ap, tc.command, tc.newer)
			if tc.want == "" && out.Len() != 0 || !strings.Contains(out.String(), tc.want) {
				t.Fatalf("output %q, want %q", out.String(), tc.want)
			}
		})
	}
}
