package core_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestContainerStoppedResumeRecoveryPreservesState(t *testing.T) {
	f, d, it := containerFixture(t)
	f.clock = f.clock.Add(2 * time.Hour)
	p := stopIntent(f, it)
	p.WasRunning = new(false)
	saveContainerIntent(t, f, p)
	if _, err := f.svc.Cancel([]string{fmt.Sprint(it.ID)}); err != nil {
		t.Fatal(err)
	}
	d.row.Running = false
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || d.row.Running || len(d.calls) != 0 || len(st.ContainerIntents) != 0 {
		t.Fatal("initially stopped recovery started container", st, d.calls, err)
	}
}

func TestContainerPreparedStopRecoveryDoesNotStart(t *testing.T) {
	f, d, it := containerFixture(t)
	p := stopIntent(f, it)
	p.Phase = "prepared"
	p.WasRunning = nil
	saveContainerIntent(t, f, p)
	if _, err := f.svc.Cancel([]string{fmt.Sprint(it.ID)}); err != nil {
		t.Fatal(err)
	}
	d.row.Running = false
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || d.row.Running || len(d.calls) != 0 || len(st.ContainerIntents) != 0 {
		t.Fatal("unsent stop was compensated", st, d.calls, err)
	}
}

func TestContainerLegacyResumeRetainsUnknownState(t *testing.T) {
	f, d, it := containerFixture(t)
	p := stopIntent(f, it)
	p.Operation = "resume"
	p.Phase = ""
	p.WasRunning = nil
	saveContainerIntent(t, f, p)
	d.row.Running = false
	rows, err := f.svc.Sweep(false)
	st, readErr := f.svc.Store.ReadSnapshot()
	if err != nil || readErr != nil || d.row.Running || len(st.ContainerIntents) != 1 || len(rows) != 1 || rows[0].Action != core.ActionRetry {
		t.Fatal(rows, st, err, readErr)
	}
}
