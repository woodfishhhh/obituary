//go:build windows

package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func queueHookInput(t *testing.T, root, path string, i int) []byte {
	t.Helper()
	patch := "*** Begin Patch\n*** Add File: " + path + "\n+artifact\n*** End Patch"
	data, err := json.Marshal(map[string]any{"hook_event_name": "PostToolUse", "session_id": fmt.Sprint("queue-", i), "cwd": root, "tool_name": "apply_patch", "tool_input": map[string]string{"input": patch}})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestConcurrentHookProcessesPreserveEveryObservation(t *testing.T) {
	if testing.Short() {
		t.Skip("starts 20 native hooks with a real Git repository and fsync")
	}
	root, store := commandRepo(t)
	if err := store.Update(func(st *core.State) error {
		st.Projects = map[string]core.ProjectState{core.ProjectKey(root): {Consent: core.ConsentApproved}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 16)
	for i := range paths {
		paths[i] = filepath.Join(root, fmt.Sprintf("debug-%02d.log", i))
		writeFixture(t, paths[i], "artifact")
	}
	release, err := core.LockFile(filepath.Join(store.Home, "store.lock"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if release != nil {
			release()
		}
	}()
	cmds, outputs := startQueuedHooks(t, root, paths)
	for i, cmd := range cmds {
		if err := cmd.Wait(); err != nil || outputs[i].Len() != 0 {
			t.Fatalf("hook %d: %s %v", i, outputs[i].String(), err)
		}
	}
	rows, err := store.DeferredObservations()
	if err != nil || len(rows) != 20 {
		t.Fatalf("contended writes lost: %d %v", len(rows), err)
	}
	for _, row := range rows {
		if row.Host != "codex" || row.Event != "PostToolUse" || core.ProjectKey(row.Project) != core.ProjectKey(root) || row.Identity == nil {
			t.Fatalf("request metadata missing: %+v", row)
		}
	}
	release()
	release = nil
	svc, err := app.Open()
	if err != nil {
		t.Fatal(err)
	}
	svc.AfterChange = nil
	if _, _, err := app.Tick(svc); err != nil {
		t.Fatal(err)
	}
	// If the isolated resident owns replay, wait on its observable result.
	waitForQueueItems(t, store, 16)
	st, err := store.ReadSnapshot()
	if err != nil || len(st.Items) != 16 || store.HasDeferredObservations() {
		t.Fatalf("queue did not drain: items=%d err=%v", len(st.Items), err)
	}
	seen := map[string]bool{}
	for _, item := range st.Items {
		if seen[item.Path] || item.Rev != 1 {
			t.Fatalf("duplicate registration: %+v", item)
		}
		seen[item.Path] = true
	}
	t.Log("20 native hook processes: 16 distinct paths plus 4 duplicate writes; 20 durable requests; 16 items, revision 1; queue drained")
}

func startQueuedHooks(t *testing.T, root string, paths []string) ([]*exec.Cmd, []*bytes.Buffer) {
	t.Helper()
	var cmds []*exec.Cmd
	var outputs []*bytes.Buffer
	for i := range 20 {
		cmd := exec.Command(binaryPath(t, "obit"), "hook", "codex")
		cmd.Stdin = bytes.NewReader(queueHookInput(t, root, paths[i%len(paths)], i))
		output := new(bytes.Buffer)
		cmd.Stdout, cmd.Stderr = output, output
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if cmd.ProcessState == nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		})
		cmds, outputs = append(cmds, cmd), append(outputs, output)
	}
	return cmds, outputs
}

func waitForQueueItems(t *testing.T, store core.Store, count int) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		st, err := store.Inspect()
		if err == nil && len(st.Items) == count && !store.HasDeferredObservations() {
			return
		}
	}
	t.Fatal("background replay did not reach the expected ledger/queue state")
}

func TestHookQueueUnwritableFailsOpen(t *testing.T) {
	if testing.Short() {
		t.Skip("runs a native hook in a real Git repository")
	}
	root, store := commandRepo(t)
	path := filepath.Join(root, "debug.log")
	writeFixture(t, path, "artifact")
	writeFixture(t, filepath.Join(store.Home, "observations"), "queue directory unavailable")
	release, err := core.LockFile(filepath.Join(store.Home, "store.lock"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	cmd := exec.Command(binaryPath(t, "obit"), "hook", "codex")
	cmd.Stdin = bytes.NewReader(queueHookInput(t, root, path, 1))
	start := time.Now()
	out, err := cmd.CombinedOutput()
	if err != nil || len(out) != 0 || time.Since(start) > 1400*time.Millisecond {
		t.Fatalf("unwritable queue blocked hook: %s %v (%s)", out, err, time.Since(start))
	}
	data, err := os.ReadFile(filepath.Join(store.Home, "hook.log"))
	if err != nil || !strings.Contains(string(data), "observations") {
		t.Fatalf("queue failure not logged: %s %v", data, err)
	}
}

func TestQueuedHookProcessLatencyReport(t *testing.T) {
	if testing.Short() {
		t.Skip("measures 10 native hooks and a one-second ledger wait")
	}
	root, store := commandRepo(t)
	path := filepath.Join(root, "debug.log")
	writeFixture(t, path, "artifact")
	release, err := core.LockFile(filepath.Join(store.Home, "store.lock"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	var samples []time.Duration
	for i := range 10 {
		cmd := exec.Command(binaryPath(t, "obit"), "hook", "codex")
		cmd.Stdin = bytes.NewReader(queueHookInput(t, root, path, i))
		start := time.Now()
		out, err := cmd.CombinedOutput()
		samples = append(samples, time.Since(start))
		if err != nil || len(out) != 0 {
			t.Fatalf("queued hook: %s %v", out, err)
		}
	}
	start := time.Now()
	_, err = store.WithDeadline(start.Add(time.Second)).ReadSnapshot()
	if err == nil {
		t.Fatal("fixture ledger was not locked")
	}
	wait := time.Since(start)
	clitest.ReportHookLatency(t, "PostToolUse/queued", "process start to exit; busy ledger; fsync enabled", samples)
	t.Logf("ledger lock wait: %s; queue p95=%s", wait, samples[len(samples)-1])
	if samples[len(samples)-1] >= wait {
		t.Fatal("queue entry took longer than the ledger wait")
	}
	rows, err := store.DeferredObservations()
	if err != nil || len(rows) != 10 {
		t.Fatal("latency samples lost requests", len(rows), err)
	}
}

func TestHookQueueReplaySurvivesKilledProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("kills a native replay process after its ledger commit")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	writeFixture(t, filepath.Join(repo, "debug.log"), "artifact")
	testenv.InitRepo(t, repo)
	home := filepath.Join(root, "data")
	t.Setenv("OBIT_HOME", home)
	t.Setenv("OBIT_CONFIG_DIR", filepath.Join(root, "config"))
	svc, err := app.Open()
	if err != nil {
		t.Fatal(err)
	}
	svc.AfterChange = nil
	if err := svc.Store.Update(func(st *core.State) error {
		st.Projects = map[string]core.ProjectState{core.ProjectKey(repo): {Consent: core.ConsentApproved}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := svc.CaptureObservations([]string{filepath.Join(repo, "debug.log")}, "killed")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Store.DeferObservations(rows); err != nil {
		t.Fatal(err)
	}
	killReplayAfterCommit(t, svc.Store, repo)
	st, err := svc.Store.ReadSnapshot()
	if err != nil || len(st.Items) != 1 || !svc.Store.HasDeferredObservations() {
		t.Fatal("kill did not interrupt acknowledgement", st, err)
	}
	expires := st.Items[0].ExpiresAt
	if err := app.Engine(svc).ReplayObservations(); err != nil {
		t.Fatal(err)
	}
	st, err = svc.Store.ReadSnapshot()
	if err != nil || len(st.Items) != 1 || st.Items[0].Rev != 1 || !st.Items[0].ExpiresAt.Equal(expires) || svc.Store.HasDeferredObservations() {
		t.Fatal("killed replay duplicated registration", st, err)
	}
	if out, err := exec.Command(binaryPath(t, "obit"), "daemon", "stop").CombinedOutput(); err != nil {
		t.Fatal(string(out), err)
	}
}

func killReplayAfterCommit(t *testing.T, store core.Store, repo string) {
	t.Helper()
	release, err := core.LockFile(filepath.Join(store.Home, "audit.lock"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	cmd := exec.Command(binaryPath(t, "obit"), "hook", "finish", "--project", repo, "--session", "killed")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		st, err := store.Inspect()
		if err == nil && len(st.Items) == 1 {
			return
		}
	}
	t.Fatal("replay did not commit before kill")
}
