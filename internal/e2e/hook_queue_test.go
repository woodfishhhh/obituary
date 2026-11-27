//go:build windows

package e2e_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/daemonipc"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"golang.org/x/sys/windows"
)

// The subprocess is a resident whose request handler fails in one chosen way:
// it exits before or after the durable queue write, never answers after it, or
// refuses the request. The hook is the real binary either way.
func runInterruptedResident(stage string) int {
	store := ledger.Store{Home: os.Getenv("OBIT_HOME")}
	_, err := daemonipc.Serve(store.Home, func(rows []ledger.Observation) error {
		switch stage {
		case "before-queue":
			os.Exit(83)
		case "after-queue":
			if err := store.AcceptObservations(rows); err != nil {
				return err
			}
			os.Exit(83)
		case "silent-after-queue":
			if err := store.AcceptObservations(rows); err != nil {
				return err
			}
			select {} // the acknowledgement never comes
		}
		return errors.New("resident refuses requests") // "refused"
	}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("ready")
	select {}
}

func TestInterruptedResidentHookReplaysExactlyOnce(t *testing.T) {
	for _, stage := range []string{"before-queue", "after-queue", "silent-after-queue", "refused"} {
		t.Run(stage, func(t *testing.T) {
			exe := binaryPath(t, "obit")
			root := t.TempDir()
			repo := filepath.Join(root, "repo")
			path := filepath.Join(repo, "debug.log")
			writeFixture(t, path, "artifact")
			testenv.InitRepo(t, repo)
			t.Chdir(repo)
			store := ledger.Store{Home: filepath.Join(root, "data")}
			t.Setenv("OBIT_HOME", store.Home)
			t.Setenv("OBIT_CONFIG_DIR", filepath.Join(root, "config"))
			autoOn(t, "")
			if err := store.Update(func(st *ledger.State) error {
				st.Projects = map[string]ledger.ProjectState{ledger.ProjectKey(repo): {Consent: ledger.ConsentApproved}}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			resident := exec.CommandContext(t.Context(), self, "ipc-crash-fixture", stage)
			out, err := resident.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			resident.Stderr = os.Stderr
			if err := resident.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if resident.ProcessState == nil {
					_ = resident.Process.Kill()
					_ = resident.Wait()
				}
			})
			if line, err := bufio.NewReader(out).ReadString('\n'); err != nil || strings.TrimSpace(line) != "ready" {
				t.Fatalf("resident startup %q: %v", line, err)
			}
			hook := exec.CommandContext(t.Context(), exe, "hook", "codex")
			hook.Stdin = bytes.NewReader(queueHookInput(t, repo, path, 1))
			if out, err := hook.CombinedOutput(); err != nil {
				t.Fatal(string(out), err)
			}
			if strings.HasPrefix(stage, "before") || stage == "after-queue" {
				if err := resident.Wait(); err == nil || resident.ProcessState.ExitCode() != 83 {
					t.Fatalf("resident never exited in handler: %v", err)
				}
			} else { // alive but silent or refusing: the hook has already queued around it
				_ = resident.Process.Kill()
				_ = resident.Wait()
			}
			queued, err := store.DeferredObservations()
			if err != nil || len(queued) != 1 {
				t.Fatalf("lost or duplicated fallback observation: %d %v", len(queued), err)
			}
			svc, err := app.Open()
			if err != nil {
				t.Fatal(err)
			}
			svc.AfterChange = nil
			svc.Store = svc.Store.WithResidentReplay()
			for range 2 {
				if err := app.Engine(svc).ReplayObservations(); err != nil {
					t.Fatal(err)
				}
			}
			st, err := store.ReadSnapshot()
			if err != nil || len(st.Items) != 1 || st.Items[0].Path != path || st.Items[0].Rev != 1 || !st.ObservationCompleted(&queued[0]) || store.HasDeferredObservations() {
				t.Fatalf("lost or repeated registration: %+v %v", st.Items, err)
			}
		})
	}
}

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
	root, store := commandRepoDirect(t)
	if err := store.Update(func(st *ledger.State) error {
		st.Projects = map[string]ledger.ProjectState{ledger.ProjectKey(root): {Consent: ledger.ConsentApproved}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 16)
	for i := range paths {
		paths[i] = filepath.Join(root, fmt.Sprintf("debug-%02d.log", i))
		writeFixture(t, paths[i], "artifact")
	}
	release, err := ledger.LockFile(filepath.Join(store.Home, "store.lock"), true)
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
		if row.Host != "codex" || row.Event != "PostToolUse" || ledger.ProjectKey(row.Project) != ledger.ProjectKey(root) || row.Identity == nil {
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

func waitForQueueItems(t *testing.T, store ledger.Store, count int) {
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
	root, store := commandRepoDirect(t)
	path := filepath.Join(root, "debug.log")
	writeFixture(t, path, "artifact")
	// The resident created the queue directory; deny adding files to it.
	clitest.DenyAccess(t, filepath.Join(store.Home, "observations"), windows.FILE_WRITE_DATA) // add file, on a directory
	release, err := ledger.LockFile(filepath.Join(store.Home, "store.lock"), true)
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
	root, store := commandRepoDirect(t)
	path := filepath.Join(root, "debug.log")
	writeFixture(t, path, "artifact")
	release, err := ledger.LockFile(filepath.Join(store.Home, "store.lock"), true)
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
	autoOn(t, "")
	svc, err := app.Open()
	if err != nil {
		t.Fatal(err)
	}
	svc.AfterChange = nil
	if err := svc.Store.Update(func(st *ledger.State) error {
		st.Projects = map[string]ledger.ProjectState{ledger.ProjectKey(repo): {Consent: ledger.ConsentApproved}}
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

func killReplayAfterCommit(t *testing.T, store ledger.Store, repo string) {
	t.Helper()
	release, err := ledger.LockFile(filepath.Join(store.Home, "audit.lock"), true)
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

// With a resident, hooks do not touch the ledger lock: a writer holding it for
// seconds does not prevent them from handing off all their writes.
// The old path waited out its full one-second lock deadline first.
func TestResidentTakesHookWritesWhileTheLedgerIsLocked(t *testing.T) {
	if testing.Short() {
		t.Skip("starts 20 native hooks with a real Git repository and fsync")
	}
	root, store := commandRepo(t)
	if err := store.Update(func(st *ledger.State) error {
		st.Projects = map[string]ledger.ProjectState{ledger.ProjectKey(root): {Consent: ledger.ConsentApproved}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 16)
	for i := range paths {
		paths[i] = filepath.Join(root, fmt.Sprintf("debug-%02d.log", i))
		writeFixture(t, paths[i], "artifact")
	}
	release, err := ledger.LockFile(filepath.Join(store.Home, "store.lock"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if release != nil {
			release()
		}
	}()
	start := time.Now()
	cmds, outputs := startQueuedHooks(t, root, paths)
	took := make([]time.Duration, len(cmds))
	for i, cmd := range cmds {
		if err := cmd.Wait(); err != nil || outputs[i].Len() != 0 {
			t.Fatalf("hook %d: %s %v", i, outputs[i].String(), err)
		}
		took[i] = time.Since(start)
	}
	slices.Sort(took)
	if median := took[len(took)/2]; median > 900*time.Millisecond {
		t.Fatalf("hooks waited for the ledger lock: median %s", median)
	}
	if _, err := os.Stat(filepath.Join(store.Home, "hook.log")); err == nil {
		data, _ := os.ReadFile(filepath.Join(store.Home, "hook.log"))
		t.Fatalf("a hook failed open: %s", data)
	}
	if st, err := store.Inspect(); err != nil || len(st.ObservationReceipts) != 0 || !store.HasDeferredObservations() {
		t.Fatalf("writes should be queued behind the ledger lock: receipts=%d err=%v", len(st.ObservationReceipts), err)
	}
	release()
	release = nil
	waitForQueueItems(t, store, 16)
	st, err := store.ReadSnapshot()
	// A timed-out acknowledgement can leave two queue records for the same token.
	if err != nil || len(st.ObservationReceipts) != len(cmds) || len(st.Items) != len(paths) {
		t.Fatalf("receipts=%d items=%d err=%v", len(st.ObservationReceipts), len(st.Items), err)
	}
	for _, item := range st.Items {
		if item.Rev != 1 {
			t.Fatalf("duplicate registration: %+v", item)
		}
	}
}

// The registration a hook handed over is told to the agent by a later callback,
// from the committed ledger: the ledger is the queue.
func TestResidentRegistrationIsToldByTheNextCallback(t *testing.T) {
	if testing.Short() {
		t.Skip("runs native hooks in a real Git repository")
	}
	root, store := commandRepo(t)
	// Every automatic registration is proposed, in a project nobody was asked about.
	writeFixture(t, filepath.Join(os.Getenv("OBIT_CONFIG_DIR"), "config.toml"), "[proposals]\nmode = \"ask\"\n")
	post := func(name string) string {
		path := filepath.Join(root, name)
		writeFixture(t, path, "artifact")
		input, err := json.Marshal(map[string]any{"hook_event_name": "PostToolUse", "session_id": "notice", "cwd": root,
			"tool_name": "Write", "tool_input": map[string]string{"file_path": path}})
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(binaryPath(t, "obit"), "hook", "claude")
		cmd.Stdin = bytes.NewReader(input)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("hook: %s %v", out, err)
		}
		return string(out)
	}
	if out := post("debug.log"); out != "" {
		t.Fatalf("the hook handed the write over and has nothing to tell yet: %s", out)
	}
	waitFor(t, func() bool {
		st, err := store.ReadSnapshot()
		return err == nil && len(st.Proposals) == 1
	})
	if out := post("second.log"); !strings.Contains(out, "obit approve") || !strings.Contains(out, "debug.log") {
		t.Fatalf("the next callback must tell the proposal: %s", out)
	}
	waitFor(t, func() bool {
		st, err := store.ReadSnapshot()
		return err == nil && len(st.Proposals) == 1 && !store.HasDeferredObservations() // the second write joins the project's proposal
	})
}

func waitFor(t *testing.T, ready func() bool) {
	t.Helper()
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if ready() {
			return
		}
	}
	home := os.Getenv("OBIT_HOME")
	log, _ := os.ReadFile(filepath.Join(home, "obitd.log"))
	hookLog, _ := os.ReadFile(filepath.Join(home, "hook.log"))
	st, _ := ledger.Store{Home: home}.Inspect()
	t.Fatalf("the resident did not reach the expected ledger state: items=%d proposals=%d\nobitd.log: %s\nhook.log: %s", len(st.Items), len(st.Proposals), log, hookLog)
}
