//go:build docker_e2e

package e2e

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/docker"
	"github.com/woodfishhhh/obituary/internal/rules"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

const containerTestImage = "busybox:1.36"

type realDockerFixture struct {
	t      *testing.T
	prefix string
	home   string
	now    time.Time
	svc    *lifecycle.Service
	engine *auto.Engine
	seq    int
}

func newRealDockerFixture(t *testing.T) *realDockerFixture {
	t.Helper()
	if _, err := os.Stat(dockerBinary); err != nil {
		t.Fatal("docker is required for the docker_e2e job")
	}
	home := t.TempDir()
	t.Setenv("OBIT_HOME", home)
	t.Setenv("OBIT_CONFIG_DIR", filepath.Join(home, "config"))
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("DO_NOT_TRACK", "1")
	t.Setenv("OBIT_TELEMETRY", "0")
	f := &realDockerFixture{
		t:      t,
		prefix: fmt.Sprintf("obit-e2e-c-%d", time.Now().UnixNano()),
		home:   home,
		now:    time.Now().UTC(),
	}
	f.svc = &lifecycle.Service{
		Docker:       docker.CLI{Executable: dockerBinary},
		Store:        ledger.Store{Home: home},
		FS:           winfs.FS{},
		Retention:    lifecycle.DefaultRetention,
		ProposalMode: lifecycle.ProposalsAsk,
		Now:          func() time.Time { return f.now },
	}
	f.engine = &auto.Engine{
		Svc:   f.svc,
		Trust: rules.TrustStore{Path: filepath.Join(home, "trusted_rules.json")},
	}
	return f
}

func (f *realDockerFixture) nextName(tag string) string {
	f.seq++
	return fmt.Sprintf("%s-%s-%d", f.prefix, tag, f.seq)
}

func (f *realDockerFixture) startContainer(name, sentinel string, extraArgs ...string) string {
	f.t.Helper()
	_, _ = runDockerAllowFailure("", "rm", "-f", name)
	f.t.Cleanup(func() { _, _ = runDockerAllowFailure("", "rm", "-f", name) })
	args := []string{"run", "-d", "--stop-signal=SIGKILL", "--name", name}
	args = append(args, extraArgs...)
	args = append(args, containerTestImage, "sh", "-c", `printf "%s" "$1" > /tmp/obit-layer.txt; exec sleep 600`, "_", sentinel)
	id := runDocker(f.t, args...)
	if len(id) != 64 {
		f.t.Fatalf("expected 64-char container ID for %s, got %q", name, id)
	}
	return id
}

func (f *realDockerFixture) createVolume(name, sentinel string) {
	f.t.Helper()
	_, _ = runDockerAllowFailure("", "volume", "rm", "-f", name)
	f.t.Cleanup(func() { _, _ = runDockerAllowFailure("", "volume", "rm", "-f", name) })
	runDocker(f.t, "volume", "create", "--", name)
	runDocker(f.t, "run", "--rm", "--mount", "type=volume,source="+name+",destination=/data", containerTestImage, "sh", "-c", `printf "%s" "$1" > /data/sentinel.txt`, "_", sentinel)
}

func containerRunning(t *testing.T, ref string) bool {
	t.Helper()
	return runDocker(t, "container", "inspect", "--format", "{{.State.Running}}", "--", ref) == "true"
}

func containerCreated(t *testing.T, ref string) time.Time {
	t.Helper()
	raw := runDocker(t, "container", "inspect", "--format", "{{.Created}}", "--", ref)
	ts, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		t.Fatalf("parse created %q: %v", raw, err)
	}
	return ts
}

func initTempGitRepo(t *testing.T) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.name", "obit-e2e"},
		{"config", "user.email", "obit-e2e@example.invalid"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return repo
}

func TestDockerContainerRealEngine(t *testing.T) {
	t.Run("SessionOwnershipAndFullID", testContainerSessionOwnershipAndFullID)
	t.Run("ExpiryStopResurrectAndManualBury", testContainerExpiryStopResurrectAndManualBury)
	t.Run("ManualRestartAndStartedAtDriftProtected", testContainerManualRestartProtected)
	t.Run("SameNameReplacementProtected", testContainerSameNameReplacementProtected)
	t.Run("PurgePreservesVolumeAndImage", testContainerPurgePreservesVolumeAndImage)
}

func testContainerSessionOwnershipAndFullID(t *testing.T) {
	f := newRealDockerFixture(t)
	repo := initTempGitRepo(t)

	preID := f.startContainer(f.nextName("pre"), "pre", "--mount", "type=bind,source="+repo+",destination=/work")
	preCreated := containerCreated(t, preID)
	f.now = preCreated.Add(50 * time.Millisecond)
	for !time.Now().UTC().After(f.now) {
		time.Sleep(10 * time.Millisecond)
	}
	if err := f.engine.Snapshot(repo, "sess-m9"); err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	bindID := f.startContainer(f.nextName("bind"), "bind", "--mount", "type=bind,source="+repo+",destination=/work")
	composeID := f.startContainer(f.nextName("compose"), "compose", "--label", "com.docker.compose.project.working_dir="+repo)
	unattrID := f.startContainer(f.nextName("unattr"), "unattr")
	autoRmName := f.nextName("autorm")
	autoRmID := f.startContainer(autoRmName, "autorm", "--rm")
	_ = autoRmID

	if _, err := f.svc.AddContainer(autoRmName, lifecycle.AddOptions{ExpiresAt: f.now.Add(time.Hour)}); !errors.Is(err, docker.ErrAutoRemove) {
		t.Fatalf("expected ErrAutoRemove for --rm container, got %v", err)
	}

	f.now = time.Now().UTC().Add(2 * time.Second)
	if _, err := f.engine.ContainerLeftovers(repo, "sess-m9"); err != nil {
		t.Fatalf("ContainerLeftovers: %v", err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 || st.Proposals[0].Kind != ledger.ProposalContainer {
		t.Fatalf("expected 1 container proposal, got proposals=%+v err=%v", st.Proposals, err)
	}
	var gotIDs []string
	for _, c := range st.Proposals[0].Candidates {
		if c.Kind != ledger.KindContainer || c.Container == nil || len(c.Container.ID) != 64 || c.Container.Target == nil || c.Container.Target.DaemonID == "" {
			t.Fatalf("invalid proposed container identity: %+v", c)
		}
		gotIDs = append(gotIDs, c.Container.ID)
	}
	slices.Sort(gotIDs)
	wantIDs := []string{bindID, composeID}
	slices.Sort(wantIDs)
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("proposed container IDs=%v, want %v (pre=%s unattr=%s)", gotIDs, wantIDs, preID, unattrID)
	}
	approved, err := f.engine.Approve([]string{fmt.Sprintf("#%d", st.Proposals[0].ID)})
	if err != nil || len(approved) != 1 || lifecycle.Failed(approved[0].Results) {
		t.Fatalf("approve proposal: %+v %v", approved, err)
	}
	st, err = f.svc.Store.Read()
	if err != nil || len(st.Items) != 2 || len(st.Proposals) != 0 {
		t.Fatalf("expected 2 registered containers after approve: items=%d proposals=%d err=%v", len(st.Items), len(st.Proposals), err)
	}
}

func testContainerExpiryStopResurrectAndManualBury(t *testing.T) {
	f := newRealDockerFixture(t)
	nameExp := f.nextName("exp")
	idExp := f.startContainer(nameExp, "layer-exp-153")
	itExp, err := f.svc.AddContainer(nameExp, lifecycle.AddOptions{ExpiresAt: f.now.Add(time.Hour)})
	if err != nil || itExp.Container == nil || itExp.Container.ID != idExp {
		t.Fatalf("AddContainer exp: item=%+v err=%v", itExp, err)
	}

	nameMan := f.nextName("manual")
	idMan := f.startContainer(nameMan, "layer-man-153")
	itMan, err := f.svc.AddContainer(nameMan, lifecycle.AddOptions{ExpiresAt: f.now.Add(24 * time.Hour)})
	if err != nil || itMan.Container == nil || itMan.Container.ID != idMan {
		t.Fatalf("AddContainer manual: item=%+v err=%v", itMan, err)
	}

	// Unexpired sweep must leave both running.
	if rows, err := f.svc.Sweep(false); err != nil || len(rows) != 0 || !containerRunning(t, idExp) || !containerRunning(t, idMan) {
		t.Fatalf("unexpired sweep touched containers: rows=%+v err=%v", rows, err)
	}

	// Item #143: explicit BuryTracked stops unexpired container with ReasonBury.
	manRows, err := f.svc.BuryTracked([]string{fmt.Sprintf("#%d", itMan.ID)})
	if err != nil || len(manRows) != 1 || manRows[0].Action != lifecycle.ActionBuried || manRows[0].Burial == nil || manRows[0].Burial.Reason != lifecycle.ReasonBury {
		t.Fatalf("manual BuryTracked: %+v %v", manRows, err)
	}
	if containerRunning(t, idMan) {
		t.Fatalf("manually buried container %s is still running", idMan)
	}

	// Advance past expiry of exp container: Sweep stops it with ReasonExpired (not rm).
	f.now = f.now.Add(2 * time.Hour)
	rows, err := f.svc.Sweep(false)
	if err != nil || len(rows) != 1 || rows[0].Action != lifecycle.ActionBuried || rows[0].Burial == nil || rows[0].Burial.Reason != lifecycle.ReasonExpired {
		t.Fatalf("expired sweep: %+v %v", rows, err)
	}
	if containerRunning(t, idExp) {
		t.Fatalf("expired container %s is still running", idExp)
	}

	// Pre-retention purge must be refused.
	early, err := f.svc.PurgeRefs([]string{fmt.Sprintf("#%d", itExp.ID)}, false)
	if err != nil || len(early) != 1 || !strings.Contains(early[0].Error, "retention has not elapsed") {
		t.Fatalf("expected retention refusal, got %+v %v", early, err)
	}

	// Resurrect within retention restarts container and preserves writable layer.
	res, err := f.svc.Resurrect([]string{fmt.Sprintf("#%d", itExp.ID)})
	if err != nil || len(res) != 1 || res[0].Action != lifecycle.ActionResurrected {
		t.Fatalf("resurrect: %+v %v", res, err)
	}
	if !containerRunning(t, idExp) {
		t.Fatalf("resurrected container %s is not running", idExp)
	}
	if got := runDocker(t, "exec", idExp, "cat", "/tmp/obit-layer.txt"); got != "layer-exp-153" {
		t.Fatalf("writable layer bytes mismatch after resurrect: %q", got)
	}
}

func testContainerManualRestartProtected(t *testing.T) {
	f := newRealDockerFixture(t)
	nameRun := f.nextName("restart-run")
	idRun := f.startContainer(nameRun, "layer-run")
	itRun, err := f.svc.AddContainer(nameRun, lifecycle.AddOptions{ExpiresAt: f.now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	nameDrift := f.nextName("restart-drift")
	idDrift := f.startContainer(nameDrift, "layer-drift")
	itDrift, err := f.svc.AddContainer(nameDrift, lifecycle.AddOptions{ExpiresAt: f.now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}

	f.now = f.now.Add(2 * time.Hour)
	if rows, err := f.svc.Sweep(false); err != nil || len(rows) != 2 {
		t.Fatalf("sweep: %+v %v", rows, err)
	}

	// User manually restarts both, and stops idDrift again so StartedAt changes.
	runDocker(t, "start", "--", idRun)
	runDocker(t, "start", "--", idDrift)
	time.Sleep(150 * time.Millisecond)
	runDocker(t, "stop", "--", idDrift)

	// Advance past retention: Tick must reconcile both as resurrected and never rm either.
	f.now = f.now.Add(8 * 24 * time.Hour)
	_, purged, err := f.svc.Tick()
	if err != nil || len(purged) != 2 {
		t.Fatalf("tick after manual restart: purged=%+v err=%v", purged, err)
	}
	for _, r := range purged {
		if r.Action != lifecycle.ActionResurrected {
			t.Fatalf("expected reclaimed container to reconcile as resurrected, got %+v", r)
		}
	}
	if !containerRunning(t, idRun) {
		t.Fatalf("manually restarted container %d (%s) should stay running", itRun.ID, idRun)
	}
	if containerRunning(t, idDrift) {
		t.Fatalf("manually restarted+stopped container %d (%s) should remain stopped", itDrift.ID, idDrift)
	}
	if _, err := runDockerAllowFailure("", "container", "inspect", "--", idDrift); err != nil {
		t.Fatalf("manually restarted+stopped container %s was removed: %v", idDrift, err)
	}
}

func testContainerSameNameReplacementProtected(t *testing.T) {
	f := newRealDockerFixture(t)
	// Case A: active tracked container replaced externally with same name before sweep.
	nameA := f.nextName("swap-active")
	idA1 := f.startContainer(nameA, "orig-a")
	if _, err := f.svc.AddContainer(nameA, lifecycle.AddOptions{ExpiresAt: f.now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	runDocker(t, "rm", "-f", "--", idA1)
	idA2 := f.startContainer(nameA, "replacement-a")
	if idA1 == idA2 {
		t.Fatal("replacement container unexpectedly kept the same ID")
	}
	f.now = f.now.Add(2 * time.Hour)
	rows, err := f.svc.Sweep(false)
	if err != nil || len(rows) != 1 || rows[0].Action != lifecycle.ActionLost {
		t.Fatalf("expected lost outcome for replaced active container, got %+v %v", rows, err)
	}
	if !containerRunning(t, idA2) {
		t.Fatalf("replacement container %s with same name was stopped or removed", idA2)
	}

	// Case B: buried container replaced externally with same name before retention purge.
	nameB := f.nextName("swap-buried")
	idB1 := f.startContainer(nameB, "orig-b")
	if _, err := f.svc.AddContainer(nameB, lifecycle.AddOptions{ExpiresAt: f.now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(2 * time.Hour)
	if rows, err := f.svc.Sweep(false); err != nil || len(rows) != 1 || rows[0].Action != lifecycle.ActionBuried {
		t.Fatalf("bury before replacement: %+v %v", rows, err)
	}
	runDocker(t, "rm", "-f", "--", idB1)
	idB2 := f.startContainer(nameB, "replacement-b")

	f.now = f.now.Add(8 * 24 * time.Hour)
	_, purged, err := f.svc.Tick()
	if err != nil || len(purged) != 1 || purged[0].Action != lifecycle.ActionLost {
		t.Fatalf("expected lost outcome for replaced buried container, got %+v %v", purged, err)
	}
	if !containerRunning(t, idB2) {
		t.Fatalf("replacement container %s with same name was removed during purge", idB2)
	}
}

func testContainerPurgePreservesVolumeAndImage(t *testing.T) {
	f := newRealDockerFixture(t)
	imageBefore := runDocker(t, "image", "inspect", "--format", "{{.Id}}", "--", containerTestImage)
	volName := f.nextName("vol")
	f.createVolume(volName, "vol-sentinel-e2e")

	cName := f.nextName("purge")
	cID := f.startContainer(cName, "layer-purge", "--mount", "type=volume,source="+volName+",destination=/data")
	if _, err := f.svc.AddContainer(cName, lifecycle.AddOptions{ExpiresAt: f.now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(2 * time.Hour)
	if rows, err := f.svc.Sweep(false); err != nil || len(rows) != 1 || rows[0].Action != lifecycle.ActionBuried {
		t.Fatalf("sweep before purge: %+v %v", rows, err)
	}

	f.now = f.now.Add(8 * 24 * time.Hour)
	_, purged, err := f.svc.Tick()
	if err != nil || len(purged) != 1 || purged[0].Action != lifecycle.ActionPurged {
		t.Fatalf("tick purge after retention: %+v %v", purged, err)
	}
	if _, err := runDockerAllowFailure("", "container", "inspect", "--", cID); err == nil {
		t.Fatalf("container %s still exists after purge", cID)
	}
	if _, err := runDockerAllowFailure("", "volume", "inspect", "--", volName); err != nil {
		t.Fatalf("named volume %s was removed by container purge: %v", volName, err)
	}
	gotVol := runDocker(t, "run", "--rm", "--mount", "type=volume,source="+volName+",destination=/data", containerTestImage, "cat", "/data/sentinel.txt")
	if gotVol != "vol-sentinel-e2e" {
		t.Fatalf("volume sentinel corrupted after container purge: %q", gotVol)
	}
	imageAfter := runDocker(t, "image", "inspect", "--format", "{{.Id}}", "--", containerTestImage)
	if imageBefore != imageAfter {
		t.Fatalf("base image ID changed: before=%s after=%s", imageBefore, imageAfter)
	}
}
