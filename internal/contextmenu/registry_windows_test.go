package contextmenu

import (
	"testing"

	"github.com/woodfishhhh/obituary/internal/regtxn"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"golang.org/x/sys/windows/registry"
)

func TestContextMenuPreservesNonStringRegistryValues(t *testing.T) {
	reg := WindowsRegistry{Root: testenv.ClassesKey(t)}
	path := `*\shell\Obituary.Mark`
	key := testenv.ClassesSubkey(t, reg.Root, path)
	if err := key.SetDWordValue("user value", 42); err != nil {
		t.Fatal(err)
	}
	for _, uninstall := range []bool{false, true} {
		if _, err := Plan(reg, `C:\obit.exe`, uninstall); err == nil {
			t.Fatal("accepted foreign DWORD")
		}
	}
	if got, _, err := key.GetIntegerValue("user value"); err != nil || got != 42 {
		t.Fatal(got, err)
	}
}

func TestRegistryCompareAndSwapPreservesValueTypes(t *testing.T) {
	for _, kind := range []string{"expand", "dword", "missing", "foreign-child"} {
		t.Run(kind, func(t *testing.T) {
			root := testenv.ClassesKey(t)
			reg := WindowsRegistry{Root: root}
			before := Tree{"": {"owned": "command"}}
			if err := reg.WriteTree("owned", before); err != nil {
				t.Fatal(err)
			}
			key := testenv.ClassesSubkey(t, root, "owned")
			var err error
			switch kind {
			case "expand":
				err = key.SetExpandStringValue("owned", "command")
			case "dword":
				err = key.SetDWordValue("owned", 1)
			case "missing":
				err = key.DeleteValue("owned")
			case "foreign-child":
				err = reg.WriteTree(`owned\foreign`, Tree{"": {"keep": "value"}})
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := reg.CompareAndSwap(Change{Path: "owned", Before: before}); err == nil {
				t.Fatal("registry boundary deleted changed tree")
			}
			if _, err := key.Stat(); err != nil {
				t.Fatalf("registry boundary deleted changed tree: %v", err)
			}
		})
	}
}

func TestRegistryWritePreservesChangedTree(t *testing.T) {
	reg := WindowsRegistry{Root: testenv.ClassesKey(t)}
	before := Tree{"": {"owned": "old"}}
	if err := reg.WriteTree("owned", before); err != nil {
		t.Fatal(err)
	}
	if err := reg.WriteTree("owned", Tree{"": {"owned": "foreign"}}); err != nil {
		t.Fatal(err)
	}
	err := reg.CompareAndSwap(Change{Path: "owned", Before: before, After: Tree{"": {"owned": "new"}}})
	got, readErr := reg.ReadTree("owned")
	if err == nil || readErr != nil || got[""]["owned"] != "foreign" {
		t.Fatalf("registry write overwrote changed tree: %+v %v %v", got, err, readErr)
	}
}

func TestRegistryDeleteRechecksEachLayer(t *testing.T) {
	root := testenv.ClassesKey(t)
	reg := WindowsRegistry{Root: root}
	if err := reg.WriteTree("owned", Tree{"": {"old": "value"}}); err != nil {
		t.Fatal(err)
	}
	err := regtxn.Do(func(tx *regtxn.Transaction) error {
		key, err := tx.Open(registry.CURRENT_USER, root+`\owned`, registry.ALL_ACCESS)
		if err != nil {
			return err
		}
		defer key.Close()
		if err := key.SetStringValue("foreign", "keep"); err != nil {
			return err
		}
		return deleteCheckedKey(tx, key, "", map[string]string{"old": "value"})
	})
	if err == nil {
		t.Fatal("registry layer was deleted without recheck")
	}
	if got, err := reg.ReadTree("owned"); err != nil || got[""]["old"] != "value" {
		t.Fatalf("registry layer was deleted without recheck: %+v %v", got, err)
	}
}
