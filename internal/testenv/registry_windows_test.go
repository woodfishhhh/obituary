package testenv_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/regtxn"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestRegistryCrashDescendantsAreCleaned(t *testing.T) {
	if testing.Short() {
		t.Skip("runs an exiting child test binary")
	}
	root := os.Getenv("OBIT_TEST_REGISTRY_ROOT")
	key := testenv.ReadTestRegistry(t, root, "")
	before, err := key.ReadSubKeyNames(-1)
	if err != nil {
		t.Fatal(err)
	}
	capture := filepath.Join(t.TempDir(), "root.txt")
	cmd := exec.Command(os.Args[0], "-test.run=^TestRegistryCrashHelper$")
	cmd.Env = append(os.Environ(), "OBIT_TEST_REGISTRY_CRASH=parent", "OBIT_TEST_REGISTRY_CAPTURE="+capture)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("registry cleanup parent: %s %v", out, err)
	}
	child, err := os.ReadFile(capture)
	if err != nil || !strings.HasPrefix(string(child), root+`\child-`) {
		t.Fatalf("crashed child escaped parent's disposable key: %s %v", child, err)
	}
	after, err := key.ReadSubKeyNames(-1)
	if err != nil || len(after) != len(before) {
		t.Fatalf("crashed descendant survived parent cleanup: %v => %v, %v", before, after, err)
	}
}

func TestRegistryCrashHelper(t *testing.T) {
	if testing.Short() {
		t.Skip("child process helper")
	}
	mode := os.Getenv("OBIT_TEST_REGISTRY_CRASH")
	if mode == "" {
		t.Skip("child process helper")
	}
	if mode == "crash" {
		if err := os.WriteFile(os.Getenv("OBIT_TEST_REGISTRY_CAPTURE"), []byte(os.Getenv("OBIT_TEST_REGISTRY_ROOT")), 0600); err != nil {
			t.Fatal(err)
		}
		os.Exit(0)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestRegistryCrashHelper$")
	cmd.Env = append(os.Environ(), "OBIT_TEST_REGISTRY_CRASH=crash")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("crashed registry descendant: %s %v", out, err)
	}
}

func TestRegistryRootInheritedByProductionChildren(t *testing.T) {
	root := os.Getenv("OBIT_TEST_REGISTRY_ROOT")
	if err := regtxn.ValidateTestRoot(root); err != nil {
		t.Fatal(err)
	}
	key := testenv.ReadTestRegistry(t, root, "")
	if _, err := key.Stat(); err != nil {
		t.Fatal(err)
	}
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "OBIT_TEST_REGISTRY_ROOT=") && entry == "OBIT_TEST_REGISTRY_ROOT="+root {
			return
		}
	}
	t.Fatal("disposable root missing from child environment")
}
