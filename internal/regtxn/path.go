package regtxn

import (
	"fmt"
	"os"
	"strings"
)

// UserPath confines subprocess test writes to disposable HKCU keys. An invalid
// override must fail before opening a registry key, including read-only plans.
func UserPath(production, child string) (string, error) {
	root, set := os.LookupEnv("OBIT_TEST_REGISTRY_ROOT")
	if !set {
		return production, nil
	}
	if err := ValidateTestRoot(root); err != nil {
		return "", err
	}
	return root + `\` + child, nil
}

func ValidateTestRoot(root string) error {
	const prefix = `Software\obituary-test-`
	if !strings.HasPrefix(root, prefix) || !testKeyComponents(strings.TrimPrefix(root, prefix)) {
		return fmt.Errorf("OBIT_TEST_REGISTRY_ROOT must be a disposable Software\\obituary-test- key: %q", root)
	}
	return nil
}

func testKeyComponents(tail string) bool {
	for _, part := range strings.Split(tail, `\`) {
		if part == "" {
			return false
		}
		for _, c := range part {
			valid := c == '-' || c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
			if !valid {
				return false
			}
		}
	}
	return true
}
