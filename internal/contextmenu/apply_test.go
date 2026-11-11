//go:build windows

package contextmenu

import (
	"errors"
	"reflect"
	"testing"

	"github.com/woodfishhhh/obituary/internal/testenv"
)

type lockedRegistry struct {
	trees  map[string]Tree
	locked bool
	calls  int
	failAt int
}

func (r *lockedRegistry) Lock() (func(), error) {
	r.locked = true
	return func() { r.locked = false }, nil
}
func (r *lockedRegistry) ReadTree(path string) (Tree, error) { return r.trees[path], nil }
func (r *lockedRegistry) CompareAndSwap(c Change) error {
	if !r.locked {
		return errors.New("installation lock released before write")
	}
	r.calls++
	if r.calls == r.failAt {
		return errors.New("injected failure")
	}
	if !reflect.DeepEqual(r.trees[c.Path], c.Before) {
		return errors.New("changed")
	}
	r.trees[c.Path] = c.After
	return nil
}

func TestApplyReceiptAndLock(t *testing.T) {
	for _, failAt := range []int{0, 2} {
		reg := &lockedRegistry{trees: map[string]Tree{}, failAt: failAt}
		plan := []Change{{Path: "first", After: Tree{"": {"owned": "one"}}}, {Path: "second", After: Tree{"": {"owned": "two"}}}}
		receipt, err := ApplyWithReceipt(reg, plan)
		if failAt == 0 {
			if err != nil || len(receipt.Applied) != 2 || len(receipt.Remaining) != 0 {
				t.Fatalf("installation lock or complete receipt lost: %+v %v", receipt, err)
			}
		} else if err == nil || !reflect.DeepEqual(receipt.Applied, []string{"first"}) || !reflect.DeepEqual(receipt.Remaining, []string{"second"}) {
			t.Fatalf("partial registry receipt lost: %+v %v", receipt, err)
		}
		if reg.locked {
			t.Fatal("installation lock leaked")
		}
	}
}

func TestContextMenuPreflightsWholePlan(t *testing.T) {
	reg := WindowsRegistry{Root: testenv.ClassesKey(t)}
	plan, err := Plan(reg, `C:\obit\obit.exe`, false)
	if err != nil {
		t.Fatal(err)
	}
	last := plan[len(plan)-1].Path
	foreign := Tree{"": {"foreign": "keep"}}
	if err := reg.WriteTree(last, foreign); err != nil {
		t.Fatal(err)
	}
	if err := Apply(reg, plan); err == nil {
		t.Fatal("stale plan accepted")
	}
	for _, c := range plan[:len(plan)-1] {
		if tree, err := reg.ReadTree(c.Path); err != nil || tree != nil {
			t.Fatalf("stale suffix wrote valid prefix: %s %+v %v", c.Path, tree, err)
		}
	}
}

type changedAfterRead struct {
	WindowsRegistry
	path string
}

func (r *changedAfterRead) ReadTree(path string) (Tree, error) {
	tree, err := r.WindowsRegistry.ReadTree(path)
	if err == nil && path == r.path {
		err = r.WriteTree(path, Tree{"": {"foreign": "keep"}})
		r.path = ""
	}
	return tree, err
}

func TestContextMenuDeleteRechecksAtRegistryBoundary(t *testing.T) {
	reg := WindowsRegistry{Root: testenv.ClassesKey(t)}
	tree := Tree{"": {"owned": "old"}, "command": {"": "command"}}
	if err := reg.WriteTree("owned", tree); err != nil {
		t.Fatal(err)
	}
	racing := &changedAfterRead{WindowsRegistry: reg, path: "owned"}
	if err := Apply(racing, []Change{{Path: "owned", Before: tree}}); err == nil {
		t.Fatal("concurrent registry value was deleted")
	}
	got, err := reg.ReadTree("owned")
	if err != nil || got[""]["foreign"] != "keep" || got["command"][""] != "command" {
		t.Fatalf("concurrent registry value was deleted: %+v %v", got, err)
	}
}
