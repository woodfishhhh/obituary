package explorer

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/testenv"
)

// Same compiled-overlay protocol as internal/cli/mcp/mcp_test.go, scoped here
// so Explorer's safety regressions can run without the unrelated MCP matrix.
func TestExplorerMutations(t *testing.T) {
	if testing.Short() {
		t.Skip("runs compiled mutation subprocesses")
	}
	// Anchors are checked on every full run so a refactor that moves a
	// target fails CI at once; OBIT_TEST_MCP_MUTATIONS=1 also compiles them.
	compile := os.Getenv("OBIT_TEST_MCP_MUTATIONS") == "1"
	var goEnv []string
	if compile {
		goEnv = testenv.MutantGoEnv(t)
	}
	for _, tc := range []struct{ name, file, from, to, test, assertion string }{
		{"explorer-authorized-child", "authorization_windows.go", "if got != want {", "if false && got != want {", "TestAuditItemLocationReplacedBeforeParent", "item location replacement was authorized"},
		{"explorer-authorization-pin", "apply_windows.go", "windows.FILE_LIST_DIRECTORY|windows.FILE_READ_ATTRIBUTES|windows.FILE_WRITE_ATTRIBUTES", "windows.FILE_READ_ATTRIBUTES|windows.FILE_WRITE_ATTRIBUTES", "TestAuditDirectoryReplacedBeforeIdentity", "Git directory replacement was authorized"},
		{"explorer-final-streams", "commit_windows.go", "if !sameStreams(current, p.streams) {", "if false && !sameStreams(current, p.streams) {", "TestLabelsFinalStreamsPreserveConcurrentEdit", "concurrent stream edit lost"},
		{"explorer-final-security", "commit_windows.go", "if !bytes.Equal(securityBytes(sd), securityBytes(p.security)) {", "if false && !bytes.Equal(securityBytes(sd), securityBytes(p.security)) {", "TestLabelsFinalSecurityPreservesConcurrentDACL", "concurrent security edit lost"},
		{"explorer-new-file-attributes", "plan_windows.go", "if p.remove && before.attrs&^allowed != 0 {", "if false && p.remove && before.attrs&^allowed != 0 {", "TestLabelsCleanupKeepsNewFileUserAttributes", "cleanup deleted customized new INI"},
		{"explorer-integrity-label", "security_windows.go", "windows.DACL_SECURITY_INFORMATION | windows.LABEL_SECURITY_INFORMATION", "windows.DACL_SECURITY_INFORMATION", "TestLabelsPreserveIntegrityLabel", "integrity label lost"},
		{"explorer-visible-security", "security_windows.go", "if ace.Header.AceType != systemMandatoryLabelACE {", "if false && ace.Header.AceType != systemMandatoryLabelACE {", "TestLabelsRefuseResourceAttributes", "unsupported security was rewritten"},
		{"explorer-never-written-target", "sync.go", "if !dir.Candidate {", "if true {", "TestSyncDropsNeverWrittenCandidates", "never-written cleanup targets retained"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, err := filepath.Abs(tc.file)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(data), tc.from) != 1 {
				t.Fatal("mutation anchor missing or ambiguous")
			}
			if !compile {
				return
			}
			dir := t.TempDir()
			replacement := filepath.Join(dir, "mutant.go")
			if err := os.WriteFile(replacement, []byte(strings.Replace(string(data), tc.from, tc.to, 1)), 0o600); err != nil {
				t.Fatal(err)
			}
			overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{path: replacement}})
			if err != nil {
				t.Fatal(err)
			}
			overlayPath := filepath.Join(dir, "overlay.json")
			if err := os.WriteFile(overlayPath, overlay, 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, "go", "test", "-overlay", overlayPath, "-run", "^"+tc.test+"$", "-count=1", ".")
			cmd.Env = append(os.Environ(), goEnv...)
			out, err := cmd.CombinedOutput()
			if err == nil || ctx.Err() != nil || strings.Contains(string(out), "[build failed]") || !strings.Contains(string(out), "--- FAIL: "+tc.test) {
				t.Fatalf("mutation needs a compiled assertion failure: %v\n%s", err, out)
			}
			if !strings.Contains(string(out), tc.assertion) {
				t.Fatalf("mutation hit another defense, not %q:\n%s", tc.assertion, out)
			}
			t.Logf("compiled mutation caught by %s: %s", tc.test, tc.assertion)
		})
	}
}
