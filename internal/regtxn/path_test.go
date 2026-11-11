//go:build windows

package regtxn_test

import (
	"os"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/regtxn"
)

func TestUserPathConfinesTestRegistry(t *testing.T) {
	for _, root := range []string{`Software\obituary-test-abc`, `Software\obituary-test-abc\nested_1`} {
		t.Run(root, func(t *testing.T) {
			t.Setenv("OBIT_TEST_REGISTRY_ROOT", root)
			for _, child := range []string{"Classes", "Run"} {
				got, err := regtxn.UserPath(`Software\real`, child)
				if err != nil || got != root+`\`+child {
					t.Fatalf("test root: %q %v", got, err)
				}
			}
		})
	}
}

func TestUserPathRejectsUnsafeTestRegistry(t *testing.T) {
	for _, root := range []string{"", `Software\Classes`, `Software\Microsoft`,
		`HKCU\Software\obituary-test-abc`, `software\obituary-test-abc`, `Software\obituary-test-`,
		`Software\obituary-test-\Classes`, `Software\obituary-test-abc\..\Classes`,
		`Software\obituary-test-abc\\Run`, `Software\obituary-test-abc/Run`, `Software\obituary-test-abc\`} {
		t.Run(root, func(t *testing.T) {
			t.Setenv("OBIT_TEST_REGISTRY_ROOT", root)
			got, err := regtxn.UserPath(`Software\real`, "Classes")
			if err == nil || got != "" || !strings.Contains(err.Error(), "OBIT_TEST_REGISTRY_ROOT") {
				t.Fatalf("unsafe root accepted: %q %v", got, err)
			}
		})
	}
}

func TestUserPathProductionDefault(t *testing.T) {
	t.Setenv("OBIT_TEST_REGISTRY_ROOT", "")
	if err := os.Unsetenv("OBIT_TEST_REGISTRY_ROOT"); err != nil {
		t.Fatal(err)
	}
	got, err := regtxn.UserPath(`Software\Classes`, "Classes")
	if err != nil || got != `Software\Classes` {
		t.Fatal(got, err)
	}
}
