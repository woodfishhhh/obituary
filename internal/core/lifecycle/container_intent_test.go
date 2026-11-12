package lifecycle_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func TestContainerIntentMustCommitBeforeAction(t *testing.T) {
	f, d, _ := containerFixture(t)
	f.clock = f.clock.Add(2 * time.Hour)
	tmp := filepath.Join(f.svc.Store.Home, "store.json.tmp")
	if err := os.Mkdir(tmp, 0700); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.Sweep(false)
	if err == nil || len(d.calls) != 0 || !d.row.Running {
		t.Fatal("uncommitted stop intent acted", rows, d.calls, err)
	}
	if err := os.Remove(tmp); err != nil {
		t.Fatal(err)
	}
}

func TestContainerIntentWithMonotonicClock(t *testing.T) {
	f, d, it := containerFixture(t)
	f.svc.Now = time.Now
	if err := f.svc.Store.Update(func(st *core.State) error { st.Items[0].ExpiresAt = time.Now().Add(-time.Hour); return nil }); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.Sweep(false)
	st, readErr := f.svc.Store.ReadSnapshot()
	if err != nil || readErr != nil || d.row.Running || st.BurialIndex(it.ID) < 0 || len(st.ContainerIntents) != 0 {
		t.Fatalf("intent changed across JSON timestamp round trip: %+v %v/%v", rows, err, readErr)
	}
}
