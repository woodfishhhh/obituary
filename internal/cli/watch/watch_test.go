package watch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/cli/proposals"
	"github.com/woodfishhhh/obituary/internal/core"
)

func TestWatchCommandsAndModifiedApproval(t *testing.T) {
	starts := clitest.IsolateRunner(t)
	root := t.TempDir()
	for name, sub := range map[string]string{"OBIT_HOME": "data", "OBIT_CONFIG_DIR": "config", "OBIT_USER_HOME": "user"} {
		t.Setenv(name, filepath.Join(root, sub))
	}
	dir := filepath.Join(root, "user", "Downloads")
	clitest.WriteFixture(t, filepath.Join(dir, "file.txt"), "fixture")
	out, _, err := clitest.Execute(t, NewCmd(), "~/Downloads", "30d", "--json")
	var policy core.WatchPolicy
	if err != nil || json.Unmarshal([]byte(out), &policy) != nil || policy.Consent != "pending" || len(*starts) != 1 {
		t.Fatalf("watch: %s %v starts=%v", out, err, *starts)
	}
	out, _, err = clitest.Execute(t, NewCmd(), "--list", "--json")
	var policies []core.WatchPolicy
	if err != nil || json.Unmarshal([]byte(out), &policies) != nil || len(policies) != 1 || policies[0].Path != dir {
		t.Fatalf("list: %s %v", out, err)
	}
	out, _, err = clitest.Execute(t, NewCmd(), "--remove", "~/Downloads", "--json")
	if err != nil {
		t.Fatalf("remove: %s %v", out, err)
	}
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	st, err := svc.Store.Read()
	if err != nil || len(st.Items) != 0 || len(st.Watches) != 0 {
		t.Fatalf("remove left policy lifespans: %+v %v", st, err)
	}
	out, _, err = clitest.Execute(t, NewCmd(), dir, "30d", "--age", "modified", "--no-daemon")
	if err != nil {
		t.Fatal(err)
	}
	st, err = svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 || len(st.Items) != 0 {
		t.Fatalf("date consent: %+v %v", st, err)
	}
	if !strings.Contains(out, fmt.Sprintf("obit approve %d", st.Proposals[0].ID)) {
		t.Fatalf("watch did not show the approval command: %s", out)
	}
	if _, _, err := clitest.Execute(t, proposals.NewDecideCmd("approve", "approve", true), fmt.Sprint(st.Proposals[0].ID)); err != nil {
		t.Fatal(err)
	}
}

func TestWatchRejectsInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{}, {"dir"}, {"--list", "dir"}, {"--list", "--remove", "dir"}, {"dir", "0d"}, {"dir", "1d", "--age", "created"}} {
		if _, _, err := clitest.Execute(t, NewCmd(), args...); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}

func TestWatchListExplainsDownloadSkips(t *testing.T) {
	clitest.IsolateRunner(t)
	root := t.TempDir()
	t.Setenv("OBIT_HOME", filepath.Join(root, "home"))
	t.Setenv("OBIT_CONFIG_DIR", filepath.Join(root, "config"))
	dir := filepath.Join(root, "downloads")
	clitest.WriteFixture(t, filepath.Join(dir, "source.td"), "ordinary source")
	if _, _, err := clitest.Execute(t, NewCmd(), dir, "1h", "--no-daemon"); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--list"}, {"--list", "--json"}} {
		out, _, err := clitest.Execute(t, NewCmd(), args...)
		if err != nil || !strings.Contains(out, "source.td") || !strings.Contains(out, "download") || !strings.Contains(out, "obit add") {
			t.Fatalf("download skip explanation missing: %s %v", out, err)
		}
	}
}

func TestWatchRemoveIDAndClearExclusionCommands(t *testing.T) {
	clitest.IsolateRunner(t)
	root := t.TempDir()
	t.Setenv("OBIT_HOME", filepath.Join(root, "data"))
	t.Setenv("OBIT_CONFIG_DIR", filepath.Join(root, "config"))
	dir := filepath.Join(root, "downloads")
	nested := filepath.Join(dir, "nested")
	clitest.WriteFixture(t, filepath.Join(nested, "keep.txt"), "keep")
	var ids []int64
	for _, path := range []string{dir, nested} {
		out, _, err := clitest.Execute(t, NewCmd(), path, "1d", "--json", "--no-daemon")
		var p core.WatchPolicy
		if err != nil || json.Unmarshal([]byte(out), &p) != nil {
			t.Fatalf("watch: %s %v", out, err)
		}
		ids = append(ids, p.ID)
	}
	if _, _, err := clitest.Execute(t, NewCmd(), "--remove-id", fmt.Sprint(ids[1]), "--json"); err != nil {
		t.Fatal(err)
	}
	out, _, err := clitest.Execute(t, NewCmd(), "--list")
	if err != nil || !strings.Contains(out, "excluded #") || !strings.Contains(out, "--clear-exclusion") {
		t.Fatalf("list hid exclusion: %s %v", out, err)
	}
	if _, _, err := clitest.Execute(t, NewCmd(), "--policy", fmt.Sprint(ids[0]), "--clear-exclusion", fmt.Sprint(ids[1])); err != nil {
		t.Fatal(err)
	}
	out, _, err = clitest.Execute(t, NewCmd(), "--list", "--json")
	var policies []core.WatchPolicy
	if err != nil || json.Unmarshal([]byte(out), &policies) != nil || len(policies[0].Exclusions) != 0 {
		t.Fatalf("clear: %s %v", out, err)
	}
	if err := os.Rename(dir, dir+"-moved"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := clitest.Execute(t, NewCmd(), "--remove-id", fmt.Sprint(ids[0])); err != nil {
		t.Fatal(err)
	}
}
