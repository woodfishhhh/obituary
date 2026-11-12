package lifecycle_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/docker"
)

type containerStub struct {
	row          docker.Container
	calls        []string
	changed      bool
	afterStop    func()
	beforeAction func(string)
	afterAction  func(string)
	missing      bool
}

func (d *containerStub) Inspect(context.Context, string) (docker.Container, error) {
	if d.beforeAction != nil {
		d.beforeAction("inspect")
	}
	if d.missing {
		return docker.Container{}, docker.ErrNotFound
	}
	return d.row, nil
}
func (d *containerStub) List(context.Context) ([]docker.Container, error) {
	return []docker.Container{d.row}, nil
}
func (d *containerStub) ListIDs(context.Context) ([]string, error) {
	if d.beforeAction != nil {
		d.beforeAction("ps")
	}
	if d.missing {
		return nil, nil
	}
	return []string{d.row.ID}, nil
}
func (d *containerStub) act(verb string, id docker.Identity) error {
	if d.beforeAction != nil {
		d.beforeAction(verb)
	}
	if !id.Same(d.row.Identity) || d.changed {
		return docker.ErrIdentity
	}
	d.calls = append(d.calls, verb)
	return nil
}
func (d *containerStub) Stop(_ context.Context, id docker.Identity, beforeSend func(docker.Container) error) error {
	if beforeSend != nil {
		if err := beforeSend(d.row); err != nil {
			return errors.Join(docker.ErrNotSent, err)
		}
	}
	if err := d.act("stop", id); err != nil {
		return err
	}
	d.row.Running = false
	if d.afterStop != nil {
		d.afterStop()
	}
	return nil
}
func (d *containerStub) Start(_ context.Context, id docker.Identity) error {
	if err := d.act("start", id); err != nil {
		return err
	}
	d.row.Running = true
	if d.afterAction != nil {
		d.afterAction("start")
	}
	return nil
}
func (d *containerStub) Remove(_ context.Context, id docker.Identity, beforeSend func([]string) error) (bool, error) {
	if d.changed {
		return false, docker.ErrIdentity
	}
	if d.row.Running {
		return true, nil
	}
	if beforeSend != nil {
		if err := beforeSend([]string{d.row.ID}); err != nil {
			return false, errors.Join(docker.ErrNotSent, err)
		}
	}
	err := d.act("rm", id)
	if err == nil {
		d.missing = true
		if d.afterAction != nil {
			d.afterAction("rm")
		}
	}
	return false, err
}

func containerFixture(t *testing.T) (*fixture, *containerStub, core.Item) {
	t.Helper()
	f := newFixture(t)
	d := &containerStub{row: docker.Container{Identity: docker.Identity{Target: &docker.Target{Endpoint: "stub", DaemonID: "stub"}, ID: strings.Repeat("a", 64), ShortID: strings.Repeat("a", 12), Name: "cache", Created: f.clock.Add(-time.Hour)}, Running: true}}
	f.svc.Docker = d
	it, err := f.svc.AddContainer("cache", core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	return f, d, it
}

func TestContainerSweepPreservesIdentityMismatch(t *testing.T) {
	f, d, _ := containerFixture(t)
	d.changed = true
	f.clock = f.clock.Add(2 * time.Hour)
	rows, err := f.svc.Sweep(false)
	st, readErr := f.svc.Store.Read()
	if err != nil || readErr != nil || len(rows) != 1 || rows[0].Action != core.ActionRetry || len(st.Items) != 1 || len(d.calls) != 0 {
		t.Fatalf("rows=%+v state=%+v calls=%v err=%v/%v", rows, st, d.calls, err, readErr)
	}
}

func TestContainerSpareAndFileIsolation(t *testing.T) {
	f, _, it := containerFixture(t)
	path := f.file("ordinary.txt")
	rows, err := f.svc.Add([]string{path}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour), OnlyNew: true})
	if err != nil || core.Failed(rows) || rows[0].Item.ID == it.ID {
		t.Fatal("container blocked file or reused ID", rows, err)
	}
	results, err := f.svc.Cancel([]string{fmt.Sprint(it.ID)})
	if err != nil || core.Failed(results) {
		t.Fatal(results, err)
	}
}

func TestContainerDoesNotBlockDirectoryBurial(t *testing.T) {
	f, _, _ := containerFixture(t)
	dir := f.dir + "/ordinary-folder"
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.Bury([]string{dir}, false, "")
	if err != nil || core.GraveFailed(rows) || len(rows) != 1 || rows[0].Action != core.ActionBuried {
		t.Fatal("container blocked file directory burial", rows, err)
	}
}

func TestContainerCallsReleaseLedger(t *testing.T) {
	f, d, it := containerFixture(t)
	d.beforeAction = func(verb string) {
		err := f.svc.Store.WithDeadline(time.Now().Add(100 * time.Millisecond)).Update(func(st *core.State) error {
			st.DataID = "hook-write"
			return nil
		})
		if err != nil {
			t.Errorf("docker %s held ledger lock: %v", verb, err)
		}
	}
	f.clock = f.clock.Add(2 * time.Hour)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Resurrect([]string{fmt.Sprint(it.ID)}); err != nil {
		t.Fatal(err)
	}
	it, err := f.svc.AddContainer("cache", core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(2 * time.Hour)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(8 * 24 * time.Hour)
	if _, err := f.svc.PurgeRefs([]string{fmt.Sprint(it.ID)}, false); err != nil {
		t.Fatal(err)
	}
}

func TestContainerInvalidIdentity(t *testing.T) {
	f, d, _ := containerFixture(t)
	d.row.ID = "short"
	_, err := f.svc.AddContainer("cache", core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if !errors.Is(err, docker.ErrIdentity) {
		t.Fatal(err)
	}
}
