//go:build windows

package ledger_test

import (
	"encoding/json"
	"testing"

	core "github.com/woodfishhhh/obituary/internal/core/ledger"
)

func TestRestoreCleanupRejectsNumericValue(t *testing.T) {
	var st core.State
	if err := json.Unmarshal([]byte(`{"version":7,"restoreCleanups":[7]}`), &st); err == nil {
		t.Fatal("numeric cleanup receipt accepted")
	}
}
