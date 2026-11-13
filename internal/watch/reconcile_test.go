package watch_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/explorer"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"github.com/woodfishhhh/obituary/internal/watch"
)

func TestReconcileTopLevelAndOfflineArrivals(t *testing.T) {
	svc, dir := fixture(t)
	write(t, filepath.Join(dir, "one.txt"))
	write(t, filepath.Join(dir, "tree", "nested.txt"))
	if _, err := svc.SetWatch(dir, "30d", core.WatchFirstSeen); err != nil {
		t.Fatal(err)
	}
	approvePolicies(t, svc)
	if err := watch.Reconcile(svc); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Store.Read()
	if err != nil || len(st.Items) != 2 {
		t.Fatalf("top-level scan: %+v %v", st.Items, err)
	}
	for _, it := range st.Items {
		if filepath.Dir(it.Path) != dir {
			t.Fatalf("registered a descendant: %+v", it)
		}
	}
	write(t, filepath.Join(dir, "offline.txt"))
	reopened := *svc
	if err := watch.Reconcile(&reopened); err != nil {
		t.Fatal(err)
	}
	current, err := reopened.Store.Read()
	if err != nil || len(current.Items) != 3 {
		t.Fatalf("offline arrival: %+v %v", current.Items, err)
	}
	if !current.Items[0].ExpiresAt.Equal(st.Items[0].ExpiresAt) {
		t.Fatal("catch-up reset an existing expiry")
	}
}

func TestReconcileSkipsLabelInfrastructure(t *testing.T) {
	testenv.WindowsOnly(t, "Explorer label projection is Windows-only")
	svc, dir := fixture(t)
	write(t, filepath.Join(dir, "report.log"))
	if _, err := svc.SetWatch(dir, "2d", core.WatchFirstSeen); err != nil {
		t.Fatal(err)
	}
	approvePolicies(t, svc)
	if err := watch.Reconcile(svc); err != nil {
		t.Fatal(err)
	}
	if err := explorer.Sync(svc, explorer.Options{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, ".OBIT-LABELS-active.TMP"))
	write(t, filepath.Join(dir, "second.log"))
	if err := watch.Reconcile(svc); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Store.Read()
	if err != nil || len(st.Items) != 2 {
		t.Fatalf("watch registered label infrastructure: %+v %v", st.Items, err)
	}
	if err := explorer.Sync(svc, explorer.Options{Enabled: true}); err != nil {
		t.Fatalf("label infrastructure blocked next sync: %v", err)
	}
}

func TestReconcileGitConsentTrackedChildrenAndLaterTracking(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	svc, dir := fixture(t)
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q")
	write(t, filepath.Join(dir, "tracked.txt"))
	write(t, filepath.Join(dir, "mixed", "tracked.txt"))
	write(t, filepath.Join(dir, "scratch.txt"))
	git("add", "tracked.txt", "mixed/tracked.txt")
	p, err := svc.SetWatch(dir, "1h", core.WatchFirstSeen)
	if err != nil || p.Consent != "pending" {
		t.Fatalf("first Git policy needs consent: %+v %v", p, err)
	}
	if err := watch.Reconcile(svc); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Store.Read()
	if err != nil || len(st.Items) != 0 || len(st.Proposals) != 1 {
		t.Fatalf("before consent: %+v %v", st, err)
	}
	results, err := (&auto.Engine{Svc: svc}).Approve([]string{fmt.Sprint(st.Proposals[0].ID)})
	if err != nil || results[0].Error != "" {
		t.Fatalf("approve: %+v %v", results, err)
	}
	if err := watch.Reconcile(svc); err != nil {
		t.Fatal(err)
	}
	st, err = svc.Store.Read()
	if err != nil || len(st.Items) != 1 || filepath.Base(st.Items[0].Path) != "scratch.txt" {
		t.Fatalf("tracked files or containing directory registered: %+v %v", st.Items, err)
	}
	git("add", "scratch.txt")
	expiry := st.Items[0].ExpiresAt
	svc.Now = func() time.Time { return expiry.Add(time.Second) }
	outcomes, err := svc.Sweep(false)
	want := core.ActionRefused
	if runtime.GOOS != "windows" {
		want = core.ActionRetry // Writer exclusion is unsupported; the tracked file stays put.
	}
	if err != nil || len(outcomes) != 1 || outcomes[0].Action != want {
		t.Fatalf("later Git tracking was not protected: %+v %v", outcomes, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "scratch.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileReplacedDirectoryNeedsNewPolicy(t *testing.T) {
	svc, dir := fixture(t)
	if _, err := svc.SetWatch(dir, "30d", core.WatchFirstSeen); err != nil {
		t.Fatal(err)
	}
	approvePolicies(t, svc)
	if err := os.Rename(dir, dir+"-old"); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "replacement.txt"))
	if err := watch.Reconcile(svc); err == nil {
		t.Fatal("replacement directory inherited the old policy")
	}
	st, err := svc.Store.Read()
	if err != nil || len(st.Items) != 0 {
		t.Fatalf("replacement registered: %+v %v", st.Items, err)
	}
}
