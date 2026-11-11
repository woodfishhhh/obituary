package testenv

import (
	"crypto/rand"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// ClassesKey creates a private test hive. Cleanup never visits Software\Classes.
func ClassesKey(t *testing.T) string {
	t.Helper()
	path := `Software\obituary-test-` + rand.Text()
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
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return err
	}
	children, err := key.ReadSubKeyNames(-1)
	key.Close()
	if err != nil {
		return err
	}
	for _, name := range children {
		if err := deleteTestKey(path + `\` + name); err != nil {
			return err
		}
	}
	return registry.DeleteKey(registry.CURRENT_USER, path)
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
