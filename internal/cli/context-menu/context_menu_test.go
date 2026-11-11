//go:build windows

package contextmenu

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
)

func TestMenuResurrectsLatestWithoutProjectAndKeepsExistingFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OBIT_HOME", filepath.Join(root, "data"))
	t.Chdir(root)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Now()
	svc.Now = func() time.Time { return clock }
	var paths []string
	for _, name := range []string{"older.txt", "newer.txt"} {
		path := filepath.Join(root, name)
		paths = append(paths, path)
		clitest.WriteFixture(t, path, name)
		rows, err := svc.Bury([]string{path}, false, "")
		if err != nil || core.GraveFailed(rows) {
			t.Fatal(rows, err)
		}
		clock = clock.Add(time.Minute)
	}
	clitest.WriteFixture(t, paths[1], "replacement")
	out, stderr, err := clitest.Execute(t, NewCmd(), "resurrect", root)
	if err != nil || stderr != "" || !strings.Contains(out, "已复活") || !strings.Contains(out, "(resurrected)") {
		t.Fatal(out, stderr, err)
	}
	if data, err := os.ReadFile(paths[1]); err != nil || string(data) != "replacement" {
		t.Fatal(string(data), err)
	}
	if _, err := os.Stat(paths[0]); !os.IsNotExist(err) {
		t.Fatal("restored older record", err)
	}
	view, err := svc.ReadGraveyard()
	if err != nil || len(view.Burials) != 1 || view.Burials[0].Path != paths[0] {
		t.Fatal(view, err)
	}
}

func TestMenuLatestRecoveryDoesNotFallBack(t *testing.T) {
	for _, mode := range []string{"scan", "committed", "manifest"} {
		t.Run(mode, func(t *testing.T) {
			svc, older, newer := clitest.BurialRetryFixture(t, mode)
			oldOpen := cmdutil.Open
			cmdutil.Open = func() (*core.Service, error) { return svc, nil }
			t.Cleanup(func() { cmdutil.Open = oldOpen })
			out, stderr, err := clitest.Execute(t, NewCmd(), "resurrect", "ignored-selection")
			if mode == "committed" && err != nil {
				t.Fatal(out, stderr, err)
			}
			if mode != "committed" && err == nil {
				t.Fatal("accepted unverifiable latest", out, stderr)
			}
			st, readErr := svc.Store.Read()
			if readErr != nil {
				t.Fatal(readErr)
			}
			for _, b := range st.Burials {
				if b.ID == older.ID {
					return
				}
			}
			t.Fatalf("fell back from #%d to #%d: %s %s %v", newer.ID, older.ID, out, stderr, err)
		})
	}
}

func TestMenuPolicyAndEmptyGraveyard(t *testing.T) {
	t.Setenv("OBIT_HOME", filepath.Join(t.TempDir(), "data"))
	folder := t.TempDir()
	clitest.WriteFixture(t, filepath.Join(folder, "keep.txt"), "keep")
	out, _, err := clitest.Execute(t, NewCmd(), "policy", folder)
	if err != nil || !strings.Contains(out, "obit approve") {
		t.Fatal(out, err)
	}
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	st, err := svc.Store.Read()
	if err != nil || len(st.Watches) != 1 || st.Watches[0].Consent != "pending" || len(st.Proposals) != 1 || len(st.Items) != 0 {
		t.Fatalf("context menu did not propose an unapproved policy: %+v %v", st, err)
	}
	if _, _, err := clitest.Execute(t, NewCmd(), "resurrect", `C:\folder`); err == nil || !strings.Contains(err.Error(), "墓地为空") {
		t.Fatal(err)
	}
	if _, _, err := clitest.Execute(t, NewCmd(), "unknown", `C:\folder`); err == nil {
		t.Fatal("accepted invalid action")
	}
}

func TestMenuLatestRejectsPendingRecovery(t *testing.T) {
	view := &core.GraveyardView{Burials: []core.Burial{{ID: 1, BuriedAt: time.Now()}}, Pending: []core.PendingGrave{{}}}
	if ref, _, err := latest(view); err == nil || ref != "" {
		t.Fatal(ref, err)
	}
}
