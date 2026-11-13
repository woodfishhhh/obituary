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
	if testing.Short() {
		t.Skip("runs compiled mutation subprocesses")
	}
	// Anchors are checked on every full run so a refactor that moves a
	// target fails CI at once; OBIT_TEST_POSIX_MUTATIONS=1 also compiles them.
	compile := os.Getenv("OBIT_TEST_POSIX_MUTATIONS") == "1"
	var goEnv []string
	if compile {
		goEnv = testenv.MutantGoEnv(t)
	}
	for i, tc := range []struct{ name, file, from, to, test, assertion string }{
		{"overlay-fid-refused-linux", "stat_linux.go", "lookup(fd, name, unix.AT_EMPTY_PATH)", "lookup(fd, name, unix.AT_EMPTY_PATH|0x200)", "TestPOSIXIdentityRefusesInodeOnlyFID", "inode-only FID authorized a durable identity"},
		{"metadata-persistent", "metadata_unix.go", "if st.Incarnation == 0 {", "if false && st.Incarnation == 0 {", "TestPOSIXMetadataRequiresPersistentIdentity", "weak metadata stamp returned"},
		{"generation-required-linux", "stat_linux.go", "return binary.BigEndian.Uint64(sum[:8]), nil", "return birth + 0*binary.BigEndian.Uint64(sum[:8]), nil", "TestPOSIXGenerationSeparatesEqualBirthTimes", "generation did not separate equal birth times"},
		{"persistent-encoding", "identity_unix.go", "binary.BigEndian.PutUint64(id[:8], st.Incarnation)", "binary.BigEndian.PutUint64(id[:8], 0)", "TestPOSIXPersistentIdentitySurvivesEdits", "missing persistent component"},
		{"registration-persistent", "../core/lifecycle/register.go", "err = c.identity.RequirePersistent()", "err = nil", "TestPOSIXMissingBirthTimeRefusesRegistration", "missing birth time registration accepted"},
		{"watch-persistent", "../core/lifecycle/watch.go", "err = id.RequirePersistent()", "err = nil", "TestPOSIXMissingBirthTimeRefusesRegistration", "missing birth time watch accepted"},
		{"watch-child-persistent", "../core/lifecycle/watch_register.go", "err = c.identity.RequirePersistent()", "err = nil", "TestPOSIXMissingBirthTimeRefusesRegistration", "missing birth time watch child accepted"},
		{"legacy-expiry", "../core/lifecycle/settle.go", "if reason == ReasonExpired {", "if false && reason == ReasonExpired {", "TestPOSIXLegacyLifespanIsRetained", "legacy identity reached grave preparation"},
		{"legacy-remark", "../core/lifecycle/register.go", "st.Items[idx].Identity.RequirePersistent() != nil", "false", "TestPOSIXLegacyLifespanIsRetained", "explicit re-mark did not replace legacy identity"},
		{"move-persistent", "move_unix.go", "hex.EncodeToString(id[:]) != want.FileID", "hex.EncodeToString(id[8:]) != want.FileID[16:]", "TestPOSIXBirthTimeCheckedAtMove", "wrong birth time moved"},
		{"move-fd-metadata", "move_unix.go", "fs.statAt(fd, \"\")", "fs.statAt(parent.fd(), name)", "TestPOSIXMoveReadsPinnedIdentity", "pinned identity was reread by path"},
		{"post-move-persistent", "move_unix.go", "after.Incarnation != before.Incarnation", "false", "TestPOSIXBirthTimeCheckedAtMove", "post-move birth time change accepted"},
		{"lookup-persistent", "locate_unix.go", "hex.EncodeToString(id[:]) == want.FileID", "hex.EncodeToString(id[8:]) == want.FileID[16:]", "TestPOSIXBirthTimeCheckedAtLookup", "lookup accepted wrong birth time"},
		{"lookup-weak", "locate_unix.go", "if err := want.RequirePersistent(); err != nil", "if err := want.RequirePersistent(); false && err != nil", "TestPOSIXBirthTimeCheckedAtLookup", "weak child lookup accepted"},
		{"restore-parent-weak", "restore_unix.go", "if err := parent.RequirePersistent(); err != nil", "if err := parent.RequirePersistent(); false && err != nil", "TestPOSIXBirthTimeCheckedAtLookup", "weak restore parent accepted"},
		{"immediate-inspection-pin", "pin_unix.go", "fs.pins[id], retained = fd, true", "retained = false", "TestPOSIXImmediateWithoutBirthTime", "immediate weak burial failed"},
		{"move-weak-needs-pin", "move_unix.go", "if _, pinned := fs.pins[want]; !pinned", "if _, pinned := fs.pins[want]; false && !pinned", "TestPOSIXImmediateWithoutBirthTime", "weak move without inspection pin accepted"},
		{"grave-persistent", "../core/lifecycle/graveyard.go", "if err := id.RequirePersistent(); err != nil", "if err := id.RequirePersistent(); false && err != nil", "TestPOSIXImmediateWithoutBirthTime", "weak grave purged without durable identity"},
		{"list-legacy-warning", "../cli/list/list.go", "return err.Error()", "return \"\"", "TestPOSIXLegacyIdentityDiagnostics", "list hid legacy identity"},
		{"doctor-legacy-warning", "identity_unix.go", "if err := id.RequirePersistent(); err != nil", "if err := id.RequirePersistent(); false && err != nil", "TestPOSIXLegacyIdentityDiagnostics", "doctor hid legacy identity"},
		{"no-replace-linux", "move_linux.go", "unix.RENAME_NOREPLACE", "0", "TestPOSIXMovesNeverReplace", "no-replace move"},
		{"no-replace-darwin", "move_darwin.go", "unix.RENAME_EXCL", "0", "TestPOSIXMovesNeverReplace", "no-replace move"},
		{"post-move-identity", "move_unix.go", "after.Ino != before.Ino || after.Incarnation != before.Incarnation", "false", "TestPOSIXMoveRaceRollback", "race was not rolled back"},
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
			if !compile {
				return
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
			cmd.Env = append(os.Environ(), goEnv...)
			out, err := cmd.CombinedOutput()
			if err == nil || ctx.Err() != nil || strings.Contains(string(out), "[build failed]") || !strings.Contains(string(out), "--- FAIL: "+tc.test) || !strings.Contains(string(out), tc.assertion) {
				t.Fatalf("mutation needs its compiled behavioral assertion %q: %v\n%s", tc.assertion, err, out)
			}
			t.Logf("compiled mutation caught: %s", tc.assertion)
		})
	}
}

func TestPOSIXMoveReadsPinnedIdentity(t *testing.T) {
	dir := t.TempDir()
	path, backup, stranger, target := filepath.Join(dir, "payload"), filepath.Join(dir, "backup"), filepath.Join(dir, "stranger"), filepath.Join(dir, "grave")
	makePOSIXPayload(t, path, false, "original")
	want := posixID(t, path)
	fs := FS{afterSourcePin: func() {
		mustPOSIXRename(t, path, backup)
		makePOSIXPayload(t, path, false, "stranger")
	}, beforeRename: func() {
		mustPOSIXRename(t, path, stranger)
		mustPOSIXRename(t, backup, path)
	}}
	if err := fs.MovePinned(path, want, target); err != nil {
		t.Fatalf("pinned identity was reread by path: %v", err)
	}
	assertPOSIXPayload(t, target, false, want, "original")
	if data, err := os.ReadFile(stranger); err != nil || string(data) != "stranger" {
		t.Fatalf("stranger changed: %q %v", data, err)
	}
}

func TestPOSIXBirthTimeCheckedAtMove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload")
	makePOSIXPayload(t, path, false, "keep")
	fs := FS{}
	want := posixID(t, path)
	if err := fs.MovePinned(path, changedBirth(want), filepath.Join(dir, "grave")); err == nil {
		t.Fatal("wrong birth time moved")
	}
	assertPOSIXPayload(t, path, false, want, "keep")
	after := false
	fs.creationID = func(value uint64) uint64 {
		if after {
			return value + 1
		}
		return value
	}
	fs.beforeRename = func() { after = true }
	if err := fs.MovePinned(path, want, filepath.Join(dir, "grave")); err == nil {
		t.Fatal("post-move birth time change accepted")
	}
	assertPOSIXPayload(t, path, false, want, "keep")
}
