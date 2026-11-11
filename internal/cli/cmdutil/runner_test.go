package cmdutil_test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func TestDeferredExplorerSync(t *testing.T) {
	for _, tc := range []struct {
		name     string
		writes   int
		resident bool
		spawnErr error
		want     int
	}{{"unchanged", 0, false, nil, 0}, {"coalesced", 2, false, nil, 1}, {"resident", 2, true, nil, 0}, {"failed-spawn", 1, false, errors.New("spawn failed"), 1}} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &core.Service{Store: core.Store{Home: t.TempDir()}, FS: winfs.FS{}}
			oldOnce, oldRunning := cmdutil.RunOnce, cmdutil.RunnerRunning
			t.Cleanup(func() { cmdutil.RunOnce, cmdutil.RunnerRunning = oldOnce, oldRunning })
			calls := 0
			cmdutil.RunnerRunning = func(string) bool { return tc.resident }
			cmdutil.RunOnce = func(home string) error {
				if home != svc.Store.Home {
					t.Fatalf("wrong background home: %q", home)
				}
				calls++
				return tc.spawnErr
			}
			finish := cmdutil.DeferExplorerSync(svc)
			for range tc.writes {
				path := filepath.Join(t.TempDir(), "debug.log")
				clitest.WriteFixture(t, path, "fixture")
				results, err := svc.Add([]string{path}, core.AddOptions{ExpiresAt: time.Now().Add(time.Hour)})
				if err != nil || len(results) != 1 || results[0].Item == nil {
					t.Fatalf("registration failed: %+v %v", results, err)
				}
			}
			if calls != 0 {
				t.Fatalf("spawned during registration: %d", calls)
			}
			finish()
			if calls != tc.want {
				t.Fatalf("background calls=%d, want %d", calls, tc.want)
			}
			if tc.spawnErr != nil {
				data, err := os.ReadFile(filepath.Join(svc.Store.Home, "hook.log"))
				if err != nil || !strings.Contains(string(data), tc.spawnErr.Error()) {
					t.Fatalf("spawn failure diagnostic: %s %v", data, err)
				}
			}
		})
	}
}

func TestRunnerTickDetectsManifestWithoutDueItem(t *testing.T) {
	for _, name := range []string{"1-pending.json", "1-broken.JSON", "1-incomplete.json.tmp", "unrelated.txt", ""} {
		t.Run(name, func(t *testing.T) {
			svc := &core.Service{Store: core.Store{Home: t.TempDir()}}
			if name != "" {
				clitest.WriteFixture(t, filepath.Join(svc.Store.Home, "graveyard", name), "{not parsed by hook")
			}
			previous := cmdutil.RunOnce
			t.Cleanup(func() { cmdutil.RunOnce = previous })
			calls := 0
			cmdutil.RunOnce = func(home string) error {
				if home != svc.Store.Home {
					t.Fatal("wrong home", home)
				}
				calls++
				return nil
			}
			cmdutil.Runner(svc).Tick()
			want := 0
			if strings.EqualFold(filepath.Ext(name), ".json") {
				want = 1
			}
			if calls != want {
				t.Fatalf("runner launches=%d, want %d", calls, want)
			}
		})
	}
}

func TestManifestRecoveryTriggerBacksOffAcrossRunners(t *testing.T) {
	svc := &core.Service{Store: core.Store{Home: t.TempDir()}}
	manifest := filepath.Join(svc.Store.Home, "graveyard", "1-broken.json")
	clitest.WriteFixture(t, manifest, "{broken")
	previous := cmdutil.RunOnce
	t.Cleanup(func() { cmdutil.RunOnce = previous })
	calls := 0
	cmdutil.RunOnce = func(string) error { calls++; return nil }
	for i := 0; i < 3; i++ {
		fresh := *svc
		cmdutil.Runner(&fresh).Tick()
	}
	if calls != 1 {
		t.Fatalf("unchanged manifest repeatedly spawned runner: %d", calls)
	}
	clitest.WriteFixture(t, filepath.Join(filepath.Dir(manifest), "2-new.json"), "{also broken")
	cmdutil.Runner(svc).Tick()
	if calls != 2 {
		t.Fatalf("new manifest suppressed: %d", calls)
	}
	changed := time.Now().Add(time.Minute)
	if err := os.Chtimes(manifest, changed, changed); err != nil {
		t.Fatal(err)
	}
	cmdutil.Runner(svc).Tick()
	if calls != 3 {
		t.Fatalf("changed manifest suppressed: %d", calls)
	}
	marker := filepath.Join(svc.Store.Home, "manifest-recovery.json")
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	var receipt map[string]any
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatal(err)
	}
	receipt["at"] = time.Now().Add(-11 * time.Minute)
	data, err = json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cmdutil.Runner(svc).Tick()
	if calls != 4 {
		t.Fatalf("unchanged manifest not retried after quiet period: %d", calls)
	}
	if err := svc.Store.Update(func(st *core.State) error {
		st.Items = append(st.Items, core.Item{ID: 3, ExpiresAt: time.Now().Add(-time.Minute)})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	cmdutil.Runner(svc).Tick()
	if calls != 5 {
		t.Fatalf("new due item suppressed by recovery backoff: %d", calls)
	}
}

func TestFailedRecoverySpawnRemainsRetryable(t *testing.T) {
	svc := &core.Service{Store: core.Store{Home: t.TempDir()}}
	clitest.WriteFixture(t, filepath.Join(svc.Store.Home, "graveyard", "1-broken.json"), "{broken")
	previous := cmdutil.RunOnce
	t.Cleanup(func() { cmdutil.RunOnce = previous })
	calls := 0
	cmdutil.RunOnce = func(string) error {
		calls++
		if calls == 1 {
			return errors.New("spawn failed")
		}
		return nil
	}
	for i := 0; i < 3; i++ {
		cmdutil.Runner(svc).Tick()
	}
	if calls != 2 {
		t.Fatalf("failed spawn suppressed retry: %d", calls)
	}
}

func TestManifestRecoveryBackoffAcrossProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	home := t.TempDir()
	clitest.WriteFixture(t, filepath.Join(home, "graveyard", "1-broken.json"), "{broken")
	for i := 0; i < 2; i++ {
		cmd := exec.Command(os.Args[0], "-test.run=^TestManifestRecoveryHelperProcess$")
		cmd.Env = append(os.Environ(), "OBIT_RECOVERY_HELPER="+home)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("helper: %s, %v", out, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(home, "spawns"))
	if err != nil || string(data) != "spawn\n" {
		t.Fatalf("processes did not share backoff: %q, %v", data, err)
	}
}

func TestManifestRecoveryHelperProcess(t *testing.T) {
	home := os.Getenv("OBIT_RECOVERY_HELPER")
	if home == "" {
		return
	}
	cmdutil.RunOnce = func(string) error {
		f, err := os.OpenFile(filepath.Join(home, "spawns"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString("spawn\n")
		return err
	}
	cmdutil.Runner(&core.Service{Store: core.Store{Home: home}}).Tick()
}
