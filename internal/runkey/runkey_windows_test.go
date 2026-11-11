package runkey_test

import (
	"testing"

	"github.com/woodfishhhh/obituary/internal/runkey"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestRunCompareAndSwapPreservesConcurrentValues(t *testing.T) {
	for _, change := range []string{"replaced", "empty", "missing", "expand", "dword"} {
		t.Run(change, func(t *testing.T) {
			root := testenv.ClassesKey(t)
			reg := runkey.RegistryForTest(root)
			before := runkey.StringValue("owned.exe --tray")
			if err := reg.CompareAndSwap("owned", runkey.Value{}, before); err != nil {
				t.Fatal(err)
			}
			key := testenv.ClassesSubkey(t, root, "")
			var err error
			switch change {
			case "replaced":
				err = key.SetStringValue("owned", "foreign.exe")
			case "empty":
				err = key.SetStringValue("owned", "")
			case "missing":
				err = key.DeleteValue("owned")
			case "expand":
				err = key.SetExpandStringValue("owned", before.Text)
			case "dword":
				err = key.SetDWordValue("owned", 42)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := reg.CompareAndSwap("owned", before, runkey.Value{}); err == nil {
				t.Fatal("concurrent Run value was deleted")
			}
			if change == "dword" {
				if got, _, err := key.GetIntegerValue("owned"); err != nil || got != 42 {
					t.Fatalf("lost DWORD: %d %v", got, err)
				}
			} else if change != "missing" {
				if _, _, err := key.GetStringValue("owned"); err != nil {
					t.Fatalf("concurrent Run value was deleted: %v", err)
				}
			}
		})
	}
}

func TestRunCompareAndSwapPresenceAndRoundTrip(t *testing.T) {
	root := testenv.ClassesKey(t)
	reg := runkey.RegistryForTest(root)
	key := testenv.ClassesSubkey(t, root, "")
	if err := key.SetStringValue("owned", ""); err != nil {
		t.Fatal(err)
	}
	if err := reg.CompareAndSwap("owned", runkey.Value{}, runkey.StringValue("new")); err == nil {
		t.Fatal("empty present Run value treated as absent")
	}
	before, err := reg.Read("owned")
	if err != nil || !before.Exists || before.Text != "" {
		t.Fatal(before, err)
	}
	if err := reg.CompareAndSwap("owned", before, runkey.StringValue("command")); err != nil {
		t.Fatal(err)
	}
	if err := reg.CompareAndSwap("owned", runkey.StringValue("command"), runkey.Value{}); err != nil {
		t.Fatal(err)
	}
	if value, err := reg.Read("owned"); err != nil || value.Exists {
		t.Fatal(value, err)
	}
}
