package integrate

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// Installed OBIT.md files are recognised by the hashes of shipped versions.
// A changed Awareness must be listed too, so the next change can still
// refresh the files this version writes.
func TestAwarenessChangeKeepsOwnership(t *testing.T) {
	sum := sha256.Sum256([]byte(Awareness))
	if current := hex.EncodeToString(sum[:]); !shippedAwareness[current] {
		t.Fatalf("Awareness changed: add %q to shippedAwareness, keeping the earlier hashes", current)
	}
}
