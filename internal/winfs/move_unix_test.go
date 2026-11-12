//go:build linux || darwin

package winfs

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/testenv"
)

// Every case must compile and hit its named behavioral assertion. Local
// cross-compilation can apply this same table before executing the test binary.
func TestPOSIXMoveMutations(t *testing.T) {
	if testing.Short() || os.Getenv("OBIT_TEST_POSIX_MUTATIONS") != "1" {
		t.Skip("set OBIT_TEST_POSIX_MUTATIONS=1 for compiled POSIX safety mutations")
	}
	for i, tc := range []struct{ name, file, from, to, test, assertion string }{
		{"no-replace-linux", "move_linux.go", "unix.RENAME_NOREPLACE", "0", "TestPOSIXMovesNeverReplace", "no-replace move"},
		{"no-replace-darwin", "move_darwin.go", "unix.RENAME_EXCL", "0", "TestPOSIXMovesNeverReplace", "no-replace move"},
		{"post-move-identity", "move_unix.go", "after.Ino != before.Ino", "false", "TestPOSIXMoveRaceRollback", "race was not rolled back"},
		{"post-move-links", "move_unix.go", "st.Nlink != 1", "false", "TestPOSIXMoveRejectsNewHardLink", "new hard link was accepted"},
		{"rollback-required", "move_unix.go", "fs.renameNoReplace(to.fd(), name, from.fd(), old)", "error(nil)", "TestPOSIXMoveRaceRollback", "not found"},
		{"rollback-no-replace", "move_unix.go", "fs.renameNoReplace(to.fd(), name, from.fd(), old)", "unix.Renameat(to.fd(), name, from.fd(), old)", "TestPOSIXQuarantineNeverPurges", "identity mismatch was not durably quarantined"},
		{"quarantine-manifest", "../core/lifecycle/move_mismatch.go", "p.burial.IdentityMismatch = detail", "p.burial.IdentityMismatch = \"\"", "TestPOSIXQuarantineNeverPurges", "identity mismatch was not durably quarantined"},
		{"quarantine-recovery", "../core/lifecycle/manifest.go", "if b.IdentityMismatch != \"\" {", "if false && b.IdentityMismatch != \"\" {", "TestPOSIXQuarantineNeverPurges", "identity mismatch was not durably quarantined"},
		{"quarantine-restore", "../core/lifecycle/graveyard.go", "if b.IdentityMismatch != \"\" {", "if false && b.IdentityMismatch != \"\" {", "TestPOSIXRestoreMismatchNeverPurges", "quarantined restore accepted"},
		{"quarantine-purge", "../core/lifecycle/purge_item.go", "if b.IdentityMismatch != \"\" {", "if false && b.IdentityMismatch != \"\" {", "TestPOSIXQuarantinedMissingGraveRetainsRecord", "quarantine record retired"},
		{"unicode-byte-budget", "../core/lifecycle/resurrect_move.go", "if runtime.GOOS != \"windows\" {", "if false && runtime.GOOS != \"windows\" {", "TestPOSIXRestoreLongUnicodeCollision", "POSIX Unicode collision did not restore"},
	} {
		if !testenv.InMutationShard(t, i) || strings.HasSuffix(tc.file, "_linux.go") && runtime.GOOS != "linux" || strings.HasSuffix(tc.file, "_darwin.go") && runtime.GOOS != "darwin" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			path, err := filepath.Abs(tc.file)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), tc.from) {
				t.Fatal("mutation anchor missing")
			}
			dir := t.TempDir()
			replacement := filepath.Join(dir, "mutant.go")
			if err := os.WriteFile(replacement, []byte(strings.ReplaceAll(string(data), tc.from, tc.to)), 0600); err != nil {
				t.Fatal(err)
			}
			overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{path: replacement}})
			if err != nil {
				t.Fatal(err)
			}
			overlayPath := filepath.Join(dir, "overlay.json")
			if err := os.WriteFile(overlayPath, overlay, 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, "go", "test", "-overlay", overlayPath, ".", "-run", "^"+tc.test+"$", "-count=1")
			out, err := cmd.CombinedOutput()
			if err == nil || ctx.Err() != nil || strings.Contains(string(out), "[build failed]") || !strings.Contains(string(out), "--- FAIL: "+tc.test) || !strings.Contains(string(out), tc.assertion) {
				t.Fatalf("mutation needs its compiled behavioral assertion %q: %v\n%s", tc.assertion, err, out)
			}
			t.Logf("compiled mutation caught: %s", tc.assertion)
		})
	}
}
