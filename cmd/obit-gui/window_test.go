//go:build windows || ((linux || darwin) && desktop)

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/gui"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func TestWindowRoutesByDataDirectory(t *testing.T) {
	root := t.TempDir()
	for _, key := range []string{"HOME", "LOCALAPPDATA", "XDG_DATA_HOME", "OBIT_CONFIG_DIR", "OBIT_USER_HOME"} {
		t.Setenv(key, root)
	}
	open := func(home string) (string, string) {
		t.Helper()
		t.Setenv("OBIT_HOME", home)
		api, err := gui.Open()
		if err != nil {
			t.Fatal(err)
		}
		opts, err := windowOptions(api, 900, 600)
		if err != nil {
			t.Fatal(err)
		}
		if opts.SingleInstanceLock.OnSecondInstanceLaunch == nil {
			t.Fatal("missing activation handler")
		}
		return opts.SingleInstanceLock.UniqueId, opts.Title
	}
	def, title := open("")
	if def != "c36c42b1-df66-43e4-b909-obituary-gui" {
		t.Fatal(def)
	}
	path, err := userdirs.Data()
	if err != nil {
		t.Fatal(err)
	}
	if id, name := open(path); id != def || name != title {
		t.Fatal(id, name)
	}
	a := filepath.Join(root, "one", "Ledger")
	b := filepath.Join(root, "two", "Ledger")
	idA, name := open(a)
	idB, nameB := open(b)
	if name == nameB {
		t.Errorf("different ledgers share title %q", name)
	}
	if idA == idB || idA == def || idB == def || !strings.Contains(name, "Ledger") {
		t.Fatalf("A=%s B=%s default=%s title=%q", idA, idB, def, name)
	}
	if id, _ := open(a + string(filepath.Separator) + "."); id != idA {
		t.Fatalf("alias=%s expected=%s", id, idA)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(a, alias); err == nil {
		if id, _ := open(alias); id != idA {
			t.Fatal("linked directory opened a second window")
		}
	} else {
		t.Logf("symlink unavailable: %v", err)
	}
	if err := os.Rename(a, a+"-moved"); err != nil {
		t.Fatal(err)
	}
	if id, _ := open(a+"-moved"); id == idA {
		t.Error("renamed ledger routed to the window still using the old path")
	}
	if id, _ := open(a); id != idA {
		t.Error("recreated data directory changed the routing key for its path")
	}
}
