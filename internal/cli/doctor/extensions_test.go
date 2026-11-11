package doctor

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	initcmd "github.com/woodfishhhh/obituary/internal/cli/init"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestDoctorExtensionsCurrentnessAndReadOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	root, executable := clitest.DoctorFixture(t)
	testenv.FakeClaude(t)
	before := clitest.TreeSnapshot(t, root)
	checks, _ := doctorChecks(t, executable)
	for _, name := range []string{"mcp", "skill"} {
		if checks[name].Status != "PASS" || !strings.Contains(checks[name].Detail, "not installed") {
			t.Fatalf("missing optional extension: %+v", checks[name])
		}
		for _, agent := range []string{"claude", "codex"} {
			want := "obit init -g --agent " + agent
			if name == "mcp" {
				want += " --mcp"
			}
			if !strings.Contains(checks[name].Detail, want) {
				t.Fatalf("doctor missing agent-specific command %q: %+v", want, checks[name])
			}
		}
	}
	if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
		t.Fatal("doctor created files")
	}
	for _, agent := range []string{"claude", "codex"} {
		if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", agent, "--yes", "--mcp", "--skill"); err != nil {
			t.Fatal(err)
		}
	}
	before = clitest.TreeSnapshot(t, root)
	checks, _ = doctorChecks(t, executable)
	if checks["mcp"].Status != "PASS" || checks["skill"].Status != "PASS" || !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
		t.Fatalf("installed/read-only: mcp=%+v skill=%+v", checks["mcp"], checks["skill"])
	}
	path := filepath.Join(root, "codex", "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	clitest.WriteFixture(t, path, strings.ReplaceAll(string(data), "obit.exe", "other.exe"))
	checks, _ = doctorChecks(t, executable)
	if checks["mcp"].Status != "FAIL" {
		t.Fatalf("altered registration accepted: %+v", checks["mcp"])
	}
}

func TestDoctorAbsentMCPWithInlineParent(t *testing.T) {
	root, executable := clitest.DoctorFixture(t)
	clitest.WriteFixture(t, filepath.Join(root, "codex", "config.toml"), "mcp_servers = {other = {command = 'user'}}\n")
	checks, _ := doctorChecks(t, executable)
	if checks["mcp"].Status != "PASS" || !strings.Contains(checks["mcp"].Detail, "not installed") {
		t.Fatalf("absent optional MCP should pass: %+v", checks["mcp"])
	}
}

func TestInitCodexCRLFShowAndDoctor(t *testing.T) {
	root, executable := clitest.DoctorFixture(t)
	if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", "codex", "--mcp", "--yes"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "codex", "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	clitest.WriteFixture(t, path, strings.ReplaceAll(string(data), "\n", "\r\n"))
	before := clitest.TreeSnapshot(t, root)
	out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", "codex", "--show")
	if err != nil || !strings.Contains(out, "mcp: installed, current") {
		t.Fatalf("CRLF show failed: %s %v", out, err)
	}
	checks, _ := doctorChecks(t, executable)
	if checks["mcp"].Status != "PASS" || !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
		t.Fatalf("CRLF doctor failed or wrote files: %+v", checks["mcp"])
	}
}
