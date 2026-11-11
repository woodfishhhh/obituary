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
