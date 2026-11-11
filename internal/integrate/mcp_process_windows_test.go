package integrate

import (
	"errors"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"golang.org/x/sys/windows"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestClaudeMCPBoundsOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	testenv.FakeClaude(t)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("OBIT_FAKE_CLAUDE_MODE", "flood")
	plan, err := PlanMCP(Scope{Global: true}, "claude", "C:/tools/obit.exe", false)
	if err != nil {
		t.Fatal(err)
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	err = Apply(plan)
	runtime.ReadMemStats(&after)
	if err == nil {
		t.Fatal("flood failure reported success")
	}
	if len(err.Error()) > 70<<10 || after.TotalAlloc-before.TotalAlloc > 4<<20 {
		t.Fatalf("Claude output exceeded memory/error bound: error=%d allocated=%d", len(err.Error()), after.TotalAlloc-before.TotalAlloc)
	}
	for _, want := range []string{"flood-prefix", "flood-suffix", "omitting"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("bounded output lost %s", want)
		}
	}
}

func TestClaudeMCPWaitDelayBoundsInheritedPipes(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	exe, _ := testenv.FakeClaude(t)
	t.Setenv("OBIT_FAKE_CLAUDE_MODE", "tree-exit")
	t.Setenv("OBIT_FAKE_CLAUDE_CHILD", filepath.Join(t.TempDir(), "child.pid"))
	start := time.Now()
	_, err := runClaude(exe, []string{"mcp", "remove", "--scope", "user", "obituary"})
	if !errors.Is(err, exec.ErrWaitDelay) || time.Since(start) > 3*time.Second {
		t.Fatalf("inherited output pipes exceeded WaitDelay: %v %v", time.Since(start), err)
	}
}

func TestClaudeMCPTimeoutKillsProcessTree(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	testenv.FakeClaude(t)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("OBIT_FAKE_CLAUDE_MODE", "tree")
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	t.Setenv("OBIT_FAKE_CLAUDE_CHILD", pidFile)
	started := claudeDeadlineWhenReady(t, pidFile, 700*time.Millisecond, 2)
	plan, err := PlanMCP(Scope{Global: true}, "claude", "C:/tools/obit.exe", false)
	if err != nil {
		t.Fatal(err)
	}
	err = Apply(plan)
	elapsed := testenv.ReadyElapsed(t, started)
	pid, parseErr := strconv.Atoi(read(t, pidFile))
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	h, openErr := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, uint32(pid))
	if openErr == nil {
		defer windows.CloseHandle(h)
		defer windows.TerminateProcess(h, 1)
		status, waitErr := windows.WaitForSingleObject(h, 500)
		if waitErr != nil || status != windows.WAIT_OBJECT_0 {
			t.Fatalf("timeout left inherited-pipe child alive: %v %v", status, waitErr)
		}
	} else if !errors.Is(openErr, windows.ERROR_INVALID_PARAMETER) {
		t.Fatal(openErr)
	}
	if err == nil || !strings.Contains(err.Error(), "deadline exceeded") || elapsed > 3*time.Second {
		t.Fatalf("process tree exceeded bounded wait: %v %v", elapsed, err)
	}
}
