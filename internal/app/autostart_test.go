//go:build windows

package app

import (
	"errors"
	"testing"

	"github.com/woodfishhhh/obituary/internal/runkey"

	"golang.org/x/sys/windows"
)

type fakeRunRegistry map[string]string

func (f fakeRunRegistry) Get(name string) (string, error) { return f[name], nil }
func (f fakeRunRegistry) Set(name, value string) error    { f[name] = value; return nil }
func (f fakeRunRegistry) Delete(name string) error        { delete(f, name); return nil }
func (f fakeRunRegistry) Lock() (func(), error)           { return func() {}, nil }
func (f fakeRunRegistry) Read(name string) (runkey.Value, error) {
	if text, ok := f[name]; ok {
		return runkey.StringValue(text), nil
	}
	return runkey.Value{}, nil
}
func (f fakeRunRegistry) CompareAndSwap(name string, before, after runkey.Value) error {
	current, _ := f.Read(name)
	if current != before {
		return errors.New("Run value changed since planning")
	}
	if after.Exists {
		f[name] = after.Text
	} else {
		delete(f, name)
	}
	return nil
}

func TestAutostartPlanOwnershipAndPreview(t *testing.T) {
	store := fakeRunRegistry{}
	exe, home, config := `C:\Program Files\讣告\obitd.exe`, `C:\Data Space\`, `C:\Config Space\`
	plan, err := PlanAutostart(store, exe, home, config, true)
	if err != nil || !plan.Pending() || len(store) != 0 {
		t.Fatalf("preview: %+v %v %v", plan, store, err)
	}
	if err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	args, err := windows.DecomposeCommandLine(store[plan.Name])
	if err != nil || len(args) != 6 || args[0] != exe || args[1] != "--tray" || args[3] != home || args[5] != config {
		t.Fatalf("argv: %q %v", args, err)
	}
	again, err := PlanAutostart(store, exe, home, config, true)
	if err != nil || again.Pending() {
		t.Fatalf("idempotence: %+v %v", again, err)
	}
	remove, err := PlanAutostart(store, exe, home, config, false)
	if err != nil {
		t.Fatal(err)
	}
	store[plan.Name] = "foreign.exe"
	if err := remove.Apply(); err == nil || store[plan.Name] != "foreign.exe" {
		t.Fatal("removed changed value")
	}
	if _, err := PlanAutostart(store, exe, home, config, false); err == nil {
		t.Fatal("accepted unowned uninstall")
	}
	store[plan.Name] = plan.After
	remove, _ = PlanAutostart(store, exe, home, config, false)
	store["OtherApplication"] = "keep"
	if err := remove.Apply(); err != nil || store["OtherApplication"] != "keep" || len(store) != 1 {
		t.Fatalf("uninstall: %v %v", store, err)
	}
}

type changedRunAfterRead struct {
	fakeRunRegistry
	change bool
}

func (r *changedRunAfterRead) CompareAndSwap(name string, before, after runkey.Value) error {
	if r.change {
		r.fakeRunRegistry[name] = "foreign.exe"
		r.change = false
	}
	return r.fakeRunRegistry.CompareAndSwap(name, before, after)
}

func TestAutostartDeleteRechecksAtRegistryBoundary(t *testing.T) {
	store := &changedRunAfterRead{fakeRunRegistry: fakeRunRegistry{}}
	p, err := PlanAutostart(store, `C:\bin\obitd.exe`, `C:\data`, `C:\cfg`, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(); err != nil {
		t.Fatal(err)
	}
	p, err = PlanAutostart(store, `C:\bin\obitd.exe`, `C:\data`, `C:\cfg`, false)
	if err != nil {
		t.Fatal(err)
	}
	store.change = true
	if err := p.Apply(); err == nil || store.fakeRunRegistry[p.Name] != "foreign.exe" {
		t.Fatalf("concurrent Run value was deleted: %+v %v", store, err)
	}
}

func TestAutostartSeparatesDataDirectories(t *testing.T) {
	store := fakeRunRegistry{}
	for _, home := range []string{`C:\one`, `C:\two`} {
		p, err := PlanAutostart(store, `C:\bin\obitd.exe`, home, `C:\cfg`, true)
		if err != nil {
			t.Fatal(err)
		}
		if err := p.Apply(); err != nil {
			t.Fatal(err)
		}
	}
	if len(store) != 2 {
		t.Fatal(store)
	}
}

func TestAutostartRecognizesRelocatedInstallationOnly(t *testing.T) {
	store := fakeRunRegistry{}
	old, err := PlanAutostart(store, `C:\old\obitd.exe`, `C:\data`, `C:\cfg`, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := old.Apply(); err != nil {
		t.Fatal(err)
	}
	moved, err := PlanAutostart(store, `C:\new\obitd.exe`, `C:\data`, `C:\cfg`, true)
	if err != nil || !moved.Pending() {
		t.Fatalf("relocated plan: %+v %v", moved, err)
	}
	if err := moved.Apply(); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{
		`C:\else.exe --tray --home C:\data --config-dir C:\cfg`,
		`C:\obitd.exe --once --home C:\data --config-dir C:\cfg`,
		`C:\obitd.exe --tray --home C:\other --config-dir C:\cfg`,
		`C:\obitd.exe --tray --home C:\data --config-dir C:\cfg --extra`,
	} {
		store[moved.Name] = command
		if _, err := PlanAutostart(store, `C:\new\obitd.exe`, `C:\data`, `C:\cfg`, false); err == nil {
			t.Fatalf("accepted foreign command %q", command)
		}
	}
}

type failingRunRegistry struct{ fakeRunRegistry }

func (f failingRunRegistry) CompareAndSwap(string, runkey.Value, runkey.Value) error {
	return errors.New("registry denied")
}

type lockedRunRegistry struct {
	fakeRunRegistry
	locked bool
}

func (r *lockedRunRegistry) Lock() (func(), error) {
	r.locked = true
	return func() { r.locked = false }, nil
}

func (r *lockedRunRegistry) CompareAndSwap(name string, before, after runkey.Value) error {
	if !r.locked {
		return errors.New("autostart lock released before write")
	}
	return r.fakeRunRegistry.CompareAndSwap(name, before, after)
}

func TestAutostartHoldsInstallationLock(t *testing.T) {
	reg := &lockedRunRegistry{fakeRunRegistry: fakeRunRegistry{}}
	p, err := PlanAutostart(reg, `C:\obitd.exe`, `C:\data`, `C:\cfg`, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(); err != nil {
		t.Fatalf("autostart installation lock missing: %v", err)
	}
	if reg.locked {
		t.Fatal("autostart installation lock leaked")
	}
}

func TestAutostartReportsWriteFailure(t *testing.T) {
	p, err := PlanAutostart(failingRunRegistry{fakeRunRegistry{}}, `C:\obitd.exe`, `C:\data`, `C:\cfg`, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(); err == nil {
		t.Fatal("hidden registry failure")
	}
}
