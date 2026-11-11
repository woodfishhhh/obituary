package testenv_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestDesktopRegistryLeakDetection(t *testing.T) {
	t.Setenv("OBIT_TEST_REGISTRY", "1")
	for _, verb := range []string{`*\shell\Obituary.New`, `Directory\shell\Obituary.Mark`,
		`Directory\Background\shell\Obituary.Policy`, `Obituary.ContextMenu\shell\01\command`, `AppUserModelId\Obituary.ContextMenu`} {
		for _, change := range []string{"add", "replace", "remove", "type", "empty-key"} {
			t.Run(verb+"/"+change, func(t *testing.T) {
				root := testenv.ClassesKey(t)
				key := testenv.ClassesSubkey(t, root, verb)
				if change != "add" {
					if err := key.SetStringValue("ObituaryManifest", "original"); err != nil {
						t.Fatal(err)
					}
				}
				var output bytes.Buffer
				ok, err := testenv.DesktopChanges(root, root+`\Run`, nil, func() {
					var err error
					switch change {
					case "add", "replace":
						err = key.SetStringValue("ObituaryManifest", "leaked test command")
					case "remove":
						err = key.DeleteValue("ObituaryManifest")
					case "type":
						err = key.SetDWordValue("ObituaryManifest", 42)
					case "empty-key":
						testenv.ClassesSubkey(t, root, verb+`\empty`)
					}
					if err != nil {
						t.Fatal(err)
					}
				}, &output)
				if err != nil || ok || !strings.Contains(output.String(), verb) {
					t.Fatalf("registry leak missed: %t %v %s", ok, err, &output)
				}
			})
		}
	}
}

func TestDesktopRunLeakDetection(t *testing.T) {
	t.Setenv("OBIT_TEST_REGISTRY", "1")
	for _, name := range []string{"Obituary-test", "oBITUARY-test", "OtherApplication"} {
		t.Run(name, func(t *testing.T) {
			root := testenv.ClassesKey(t)
			key := testenv.ClassesSubkey(t, root, "Run")
			var output bytes.Buffer
			ok, err := testenv.DesktopChanges(root, root+`\Run`, nil, func() {
				if err := key.SetStringValue(name, "test.exe"); err != nil {
					t.Fatal(err)
				}
			}, &output)
			want := name == "OtherApplication"
			if err != nil || ok != want || !want && !strings.Contains(output.String(), name) {
				t.Fatalf("Run snapshot: %t %v %s", ok, err, &output)
			}
		})
	}
}

func TestDesktopShortcutLeakDetection(t *testing.T) {
	t.Setenv("OBIT_TEST_REGISTRY", "1")
	for _, change := range []string{"add", "remove", "size", "time", "unchanged"} {
		t.Run(change, func(t *testing.T) {
			root := testenv.ClassesKey(t)
			path := filepath.Join(t.TempDir(), "讣告 Obituary.lnk")
			if change != "add" {
				writeShortcutFixture(t, path, "original")
			}
			var output bytes.Buffer
			ok, err := testenv.DesktopChanges(root, root+`\Run`, []string{path}, func() {
				switch change {
				case "add", "size":
					writeShortcutFixture(t, path, "changed shortcut fixture")
				case "remove":
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				case "time":
					future := time.Now().Add(time.Hour)
					if err := os.Chtimes(path, future, future); err != nil {
						t.Fatal(err)
					}
				}
			}, &output)
			if err != nil || ok != (change == "unchanged") || !ok && !strings.Contains(output.String(), path) {
				t.Fatalf("shortcut snapshot: %t %v %s", ok, err, &output)
			}
		})
	}
}

func writeShortcutFixture(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}
