package testenv

import (
	"crypto/rand"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/regtxn"
	"golang.org/x/sys/windows/registry"
)

var processRegistryRoot string

// Nested test binaries use a child key so the parent also cleans up children
// deliberately killed by crash tests. Production children inherit the same root.
func isolateRegistry() (func() error, error) {
	root := `Software\obituary-test-` + rand.Text()
	if parent := os.Getenv("OBIT_TEST_REGISTRY_ROOT"); regtxn.ValidateTestRoot(parent) == nil {
		key, err := registry.OpenKey(registry.CURRENT_USER, parent, registry.READ)
		if err == nil {
			key.Close()
			root = parent + `\child-` + rand.Text()
		}
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, root, registry.ALL_ACCESS)
	if err != nil {
		return nil, err
	}
	key.Close()
	cleanup := func() error { return deleteTestKey(root) }
	if err := os.Setenv("OBIT_TEST_REGISTRY_ROOT", root); err != nil {
		return nil, errors.Join(err, cleanup())
	}
	processRegistryRoot = root
	return cleanup, nil
}

// ClassesKey creates a private test hive. Cleanup never visits Software\Classes.
func ClassesKey(t *testing.T) string {
	t.Helper()
	if os.Getenv("OBIT_TEST_REGISTRY") != "1" {
		t.Skip("set OBIT_TEST_REGISTRY=1 for disposable registry integration tests")
	}
	path := processRegistryRoot + `\fixture-` + rand.Text()
	if err := regtxn.ValidateTestRoot(path); err != nil {
		t.Fatal(err)
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, path+`\Classes`, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	key.Close()
	t.Cleanup(func() {
		if err := deleteTestKey(path); err != nil {
			t.Errorf("remove registry fixture %s: %v", path, err)
		}
	})
	return path + `\Classes`
}

func deleteTestKey(path string) error {
	if err := regtxn.ValidateTestRoot(path); err != nil {
		return err
	}
	return deleteTestTree(path)
}

func deleteTestTree(path string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.ENUMERATE_SUB_KEYS)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	children, err := key.ReadSubKeyNames(-1)
	key.Close()
	if err != nil {
		return err
	}
	for _, name := range children {
		if err := deleteTestTree(path + `\` + name); err != nil {
			return err
		}
	}
	return registry.DeleteKey(registry.CURRENT_USER, path)
}

// ReadTestRegistry opens only an already-created disposable root read-only.
func ReadTestRegistry(t *testing.T, root, child string) registry.Key {
	t.Helper()
	if err := regtxn.ValidateTestRoot(root); err != nil {
		t.Fatal(err)
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, root+`\`+child, registry.READ)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { key.Close() })
	return key
}

// ClassesSubkey opens or creates path below a ClassesKey root, so tests that
// need raw values (DWORD, binary) never call the registry outside testenv.
func ClassesSubkey(t *testing.T, root, path string) registry.Key {
	t.Helper()
	if !strings.HasPrefix(root, `Software\obituary-test-`) {
		t.Fatalf("not a disposable test key: %s", root)
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, root+`\`+path, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { key.Close() })
	return key
}
