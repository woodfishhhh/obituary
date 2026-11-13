package integrate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestClaudeMCPRecoveryConfirmsConcurrentTarget(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, observation := range []int{1, 2} {
		t.Run(strconv.Itoa(observation), func(t *testing.T) {
			testenv.FakeClaude(t)
			t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
			t.Setenv("OBIT_FAKE_CLAUDE_MODE", "remove-committed-error")
			path := filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), ".claude.json")
			writeIntegrationFile(t, path, testenv.NativeFixture(`{"mcpServers":{"obituary":{"command":"C:/old/obit.exe","args":["mcp"]}}}`))
			plan, err := PlanMCP(Scope{Global: true}, "claude", testenv.NativeFixture("C:/new/obit.exe"), false)
			if err != nil {
				t.Fatal(err)
			}
			readEntry := readClaudeEntry
			t.Cleanup(func() { readClaudeEntry = readEntry })
			missing := 0
			readClaudeEntry = func(p string) (json.RawMessage, error) {
				raw, err := readEntry(p)
				if err == nil && len(raw) == 0 {
					missing++
					if missing == observation {
						writeIntegrationFile(t, path, `{"mcpServers":{"obituary":`+string(plan[0].claude.want)+`}}`)
					}
				}
				return raw, err
			}
			next := filepath.Join(t.TempDir(), "following-install")
			plan = append(plan, Change{Path: next, Action: "create", data: []byte("continued")})
			if err := Apply(plan); err != nil {
				t.Fatalf("recovery target confirmation blocked install: %v", err)
			}
			entry, err := readEntry(path)
			if err != nil || !sameClaudeEntry(entry, plan[0].claude.want) || read(t, next) != "continued" {
				t.Fatalf("target or following installation lost: %s %v", entry, err)
			}
		})
	}
}

func TestClaudeManualCommandNativeArgumentFidelity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, value string
		modern      bool
	}{{"quote", `a"b=c d`, true}, {"space", "a b=c d", false}, {"ordinary", "value", false}} {
		t.Run(tc.name, func(t *testing.T) {
			entry, err := json.Marshal(map[string]any{"command": testenv.NativeFixture("C:/tools/obit.exe"), "args": []string{"mcp"}, "env": map[string]string{"DATA": tc.value}})
			if err != nil {
				t.Fatal(err)
			}
			got := CommandText(claudeAddJSON(entry))
			if tc.modern {
				if !strings.Contains(got, "'claude.exe' 'mcp' 'add-json'") || !strings.Contains(got, "PowerShell 7.3 or newer") || !strings.Contains(got, "$PSNativeCommandArgumentPassing") || !strings.Contains(got, string(entry)) {
					t.Fatalf("manual command loses native argument fidelity: %s", got)
				}
			} else if !strings.Contains(got, "'add'") || !strings.Contains(got, "'DATA="+tc.value+"'") || strings.Contains(got, "PowerShell 7.3") {
				t.Fatalf("ordinary manual env command regressed: %s", got)
			}
		})
	}
	for _, arg := range []string{"", `a"b`} {
		entry, _ := json.Marshal(map[string]any{"command": testenv.NativeFixture("C:/tools/obit.exe"), "args": []string{arg}})
		if _, ok := claudeManualAdd(entry); ok {
			t.Fatalf("legacy PowerShell cannot preserve argument %q", arg)
		}
	}
}

func TestClaudeMCPRemoveObservesConcurrentTarget(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, mode := range []string{"remove-new", "remove-new-error"} {
		t.Run(mode, func(t *testing.T) {
			_, log := testenv.FakeClaude(t)
			t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
			t.Setenv("OBIT_FAKE_CLAUDE_MODE", mode)
			path := filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), ".claude.json")
			writeIntegrationFile(t, path, testenv.NativeFixture(`{"mcpServers":{"obituary":{"command":"C:/old/obit.exe","args":["mcp"]}}}`))
			plan, err := PlanMCP(Scope{Global: true}, "claude", testenv.NativeFixture("C:/new/obit.exe"), false)
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("OBIT_FAKE_CLAUDE_NEW_ENTRY", string(plan[0].claude.want))
			next := filepath.Join(t.TempDir(), "following-install")
			plan = append(plan, Change{Path: next, Action: "create", data: []byte("continued")})
			err = Apply(plan)
			entry, readErr := readClaudeEntry(path)
			if err != nil || readErr != nil || !sameClaudeEntry(entry, plan[0].claude.want) {
				t.Fatalf("concurrent target after remove not accepted: %s %v %v", entry, readErr, err)
			}
			if calls := claudeCalls(t, log); len(calls) != 2 || calls[1][1] != "remove" {
				t.Fatalf("concurrent target was redundantly added: %v", calls)
			}
			if got := read(t, next); got != "continued" {
				t.Fatalf("concurrent target blocked subsequent installation: %s", got)
			}
		})
	}
}

func TestClaudeMCPEntryConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, changeEntry := range []bool{false, true} {
		t.Run(strconv.FormatBool(changeEntry), func(t *testing.T) {
			_, log := testenv.FakeClaude(t)
			t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
			plan, err := PlanMCP(Scope{Global: true}, "claude", testenv.NativeFixture("C:/new/obit.exe"), false)
			if err != nil {
				t.Fatal(err)
			}
			current := `{"runtime":42,"mcpServers":{"other":{"command":"keep"}}}`
			if changeEntry {
				current = testenv.NativeFixture(`{"mcpServers":{"obituary":{"command":"C:/user/obit.exe","args":["mcp"]}}}`)
			}
			writeIntegrationFile(t, plan[0].Path, current)
			err = Apply(plan)
			if !changeEntry {
				if err != nil || !strings.Contains(read(t, plan[0].Path), `"runtime":42`) {
					t.Fatalf("unrelated runtime update blocked or lost: %v", err)
				}
			} else {
				if err == nil || read(t, plan[0].Path) != current {
					t.Fatalf("changed obituary entry overwritten: %v", err)
				}
				if _, err := os.Stat(log); !os.IsNotExist(err) {
					t.Fatal("changed obituary entry invoked CLI")
				}
			}
		})
	}
}

func TestClaudeMCPRefreshPreservesFullEntry(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	testenv.FakeClaude(t)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	path := filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), ".claude.json")
	old := testenv.NativeFixture(`{"type":"stdio","command":"C:/old/obit.exe","args":["mcp"],"env":{"OBIT_HOME":"D:/another ledger","CACHE":"$cache %USER% ' &"},"custom":{"n":42}}`)
	writeIntegrationFile(t, path, `{"mcpServers":{"obituary":`+old+`}}`)
	plan, err := PlanMCP(Scope{Global: true}, "claude", testenv.NativeFixture("C:/new/obit.exe"), false)
	if err != nil {
		t.Fatal(err)
	}
	err = Apply(plan)
	entry, parseErr := claudeEntry([]byte(read(t, path)))
	if parseErr != nil || !sameJSON(entry, []byte(strings.Replace(old, testenv.NativeFixture("C:/old/obit.exe"), testenv.NativeFixture("C:/new/obit.exe"), 1))) {
		t.Fatalf("refresh lost user entry fields: %s %v %v", entry, err, parseErr)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestClaudeMCPVerifiesFullEntry(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	testenv.FakeClaude(t)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("OBIT_FAKE_CLAUDE_MODE", "drop-env")
	path := filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), ".claude.json")
	writeIntegrationFile(t, path, testenv.NativeFixture(`{"mcpServers":{"obituary":{"type":"stdio","command":"C:/old/obit.exe","args":["mcp"],"env":{"OBIT_HOME":"D:/ledger"}}}}`))
	plan, err := PlanMCP(Scope{Global: true}, "claude", testenv.NativeFixture("C:/new/obit.exe"), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err == nil || !strings.Contains(err.Error(), "verification") {
		t.Fatalf("lost env reported success: %v", err)
	}
}

func TestClaudeMCPRestoresFailedRefresh(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, mode := range []string{"fail-refresh", "fail-add"} {
		t.Run(mode, func(t *testing.T) {
			_, log := testenv.FakeClaude(t)
			t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
			t.Setenv("OBIT_FAKE_CLAUDE_MODE", mode)
			t.Setenv("OBIT_FAKE_CLAUDE_OLD_COMMAND", testenv.NativeFixture("C:/old/obit.exe"))
			path := filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), ".claude.json")
			old := testenv.NativeFixture(`{"command":"C:/old/obit.exe","args":["mcp"],"env":{"OBIT_HOME":"D:/ledger"}}`)
			writeIntegrationFile(t, path, `{"runtime":42,"mcpServers":{"obituary":`+old+`}}`)
			plan, err := PlanMCP(Scope{Global: true}, "claude", testenv.NativeFixture("C:/new/obit.exe"), false)
			if err != nil {
				t.Fatal(err)
			}
			err = Apply(plan)
			if err == nil {
				t.Fatal("failed refresh reported success")
			}
			entry, parseErr := claudeEntry([]byte(read(t, path)))
			if mode == "fail-refresh" {
				if parseErr != nil || !sameJSON(entry, []byte(old)) {
					t.Fatalf("failed refresh lost old registration: %s %v", entry, err)
				}
				if !strings.Contains(err.Error(), "restored") {
					t.Fatalf("restoration not reported: %v", err)
				}
			} else if len(entry) != 0 || !strings.Contains(err.Error(), "automatic recovery failed") || !strings.Contains(err.Error(), CommandText([]string{"claude", "mcp", "add-json", "--scope", "user", "obituary", old})) {
				t.Fatalf("failed recovery missing manual restoration: %s %v", entry, err)
			}
			if !strings.Contains(err.Error(), "already applied:") || !strings.Contains(err.Error(), "remove") || len(claudeCalls(t, log)) != 4 {
				t.Fatalf("partial CLI steps not reported: %v", err)
			}
		})
	}
}

func TestClaudeMCPRecoveryUsesObservedState(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, mode := range []string{"remove-committed-error", "remove-committed-timeout", "remove-committed-waitdelay", "remove-unreadable", "restore-committed-error", "fail-add", "add-committed-error"} {
		t.Run(mode, func(t *testing.T) {
			_, log := testenv.FakeClaude(t)
			t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
			t.Setenv("OBIT_FAKE_CLAUDE_MODE", mode)
			t.Setenv("OBIT_FAKE_CLAUDE_OLD_COMMAND", testenv.NativeFixture("C:/old/obit.exe"))
			var started <-chan time.Time
			if mode == "remove-committed-timeout" {
				ready := filepath.Join(t.TempDir(), "removed.ready")
				t.Setenv("OBIT_FAKE_CLAUDE_READY", ready)
				started = claudeDeadlineWhenReady(t, ready, 700*time.Millisecond, 2)
			}
			path := filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), ".claude.json")
			before := testenv.NativeFixture(`{"command":"C:/old/obit.exe","args":["mcp"],"env":{"OBIT_HOME":"D:/ledger"},"custom":42}`)
			writeIntegrationFile(t, path, `{"runtime":42,"mcpServers":{"obituary":`+before+`}}`)
			plan, err := PlanMCP(Scope{Global: true}, "claude", testenv.NativeFixture("C:/new/obit.exe"), false)
			if err != nil {
				t.Fatal(err)
			}
			err = Apply(plan)
			if started != nil {
				testenv.ReadyElapsed(t, started)
			}
			entry, readErr := readClaudeEntry(path)
			switch mode {
			case "add-committed-error":
				if err != nil || readErr != nil || !sameClaudeEntry(entry, plan[0].claude.want) {
					t.Fatalf("committed new registration misreported: %s %v %v", entry, readErr, err)
				}
			case "remove-unreadable":
				if err == nil || !strings.Contains(err.Error(), "cannot confirm") || !strings.Contains(err.Error(), CommandText(claudeAddJSON([]byte(before)))) || len(claudeCalls(t, log)) != 2 || strings.Contains(err.Error(), "already applied:") {
					t.Fatalf("unknown state lacks recovery command or was mutated: %v", err)
				}
			case "fail-add":
				if err == nil || len(entry) != 0 || !strings.Contains(err.Error(), "automatic recovery failed") || !strings.Contains(err.Error(), CommandText(claudeAddJSON([]byte(before)))) {
					t.Fatalf("missing recovery was misreported: %s %v", entry, err)
				}
			default:
				if readErr != nil || !sameClaudeEntry(entry, []byte(before)) {
					t.Fatalf("committed removal did not restore old entry: %s %v %v", entry, readErr, err)
				}
				if err == nil || !strings.Contains(err.Error(), "old registration restored") || strings.Contains(err.Error(), "automatic recovery failed") || !strings.Contains(err.Error(), "already applied:") {
					t.Fatalf("committed restoration misreported: %v", err)
				}
			}
		})
	}
}

func TestClaudeMCPManualGuidanceIsEnglish(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, mode := range []string{"missing", "old-version", "failed-restore"} {
		t.Run(mode, func(t *testing.T) {
			testenv.FakeClaude(t)
			t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
			switch mode {
			case "missing":
				t.Setenv("PATH", t.TempDir())
			case "old-version":
				t.Setenv("OBIT_FAKE_CLAUDE_VERSION", "2.1.317 (Claude Code)")
			case "failed-restore":
				t.Setenv("OBIT_FAKE_CLAUDE_MODE", "fail-add")
				writeIntegrationFile(t, filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), ".claude.json"), testenv.NativeFixture(`{"mcpServers":{"obituary":{"command":"C:/old/obit.exe","args":["mcp"]}}}`))
			}
			plan, err := PlanMCP(Scope{Global: true}, "claude", testenv.NativeFixture("C:/new/obit.exe"), false)
			if err != nil {
				t.Fatal(err)
			}
			err = Apply(plan)
			if err == nil || !strings.Contains(err.Error(), "close all Claude Code sessions") || !strings.Contains(err.Error(), "PowerShell") {
				t.Fatalf("English manual guidance missing: %v", err)
			}
			for _, old := range []string{"关闭", "请升级", "旧注册", "自动恢复失败"} {
				if strings.Contains(err.Error(), old) {
					t.Fatalf("non-English diagnostic: %v", err)
				}
			}
		})
	}
}

func TestClaudeStableMCPUpgradeGuidance(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	_, log := testenv.FakeClaude(t)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("OBIT_FAKE_CLAUDE_VERSION", "2.1.311 (Claude Code)")
	plan, err := PlanMCP(Scope{Global: true}, "claude", testenv.NativeFixture("C:/tools/obit.exe"), false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan[0].Detail, ClaudeUpgradeHint) {
		t.Fatal("manual plan omits channel guidance")
	}
	err = Apply(plan)
	if err == nil {
		t.Fatal("stable below config-lock floor accepted")
	}
	for _, text := range []string{"2.1.318", "stable", "latest", "autoUpdatesChannel", "claude update", "close all Claude Code sessions", "PowerShell"} {
		if !strings.Contains(err.Error(), text) {
			t.Fatalf("missing %q: %v", text, err)
		}
	}
	data, readErr := os.ReadFile(log)
	if readErr != nil || strings.Contains(string(data), "mcp") {
		t.Fatalf("old version invoked MCP: %s %v", data, readErr)
	}
}

func TestClaudeMCPRecoveryPreservesConcurrentEntry(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	_, log := testenv.FakeClaude(t)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("OBIT_FAKE_CLAUDE_MODE", "concurrent-add")
	path := filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), ".claude.json")
	writeIntegrationFile(t, path, testenv.NativeFixture(`{"mcpServers":{"obituary":{"command":"C:/old/obit.exe","args":["mcp"]}}}`))
	plan, err := PlanMCP(Scope{Global: true}, "claude", testenv.NativeFixture("C:/new/obit.exe"), false)
	if err != nil {
		t.Fatal(err)
	}
	err = Apply(plan)
	entry, readErr := readClaudeEntry(path)
	if err == nil || readErr != nil || !strings.Contains(string(entry), "user-tool") || len(claudeCalls(t, log)) != 3 {
		t.Fatalf("recovery overwrote concurrent entry: %s %v %v", entry, readErr, err)
	}
}

func TestCommandTextPowerShellLiterals(t *testing.T) {
	t.Parallel()
	args := []string{"claude", "mcp", "add-json", "--scope", "user", "obituary", testenv.NativeFixture(`{"command":"C:/space %USERNAME% $cache ' &/obit.exe","args":["mcp"]}`)}
	want := testenv.NativeFixture(`& 'claude' 'mcp' 'add' '--scope' 'user' '--transport' 'stdio' 'obituary' '--' 'C:/space %USERNAME% $cache '' &/obit.exe' 'mcp'`)
	if got := CommandText(args); got != want {
		t.Fatalf("PowerShell literals changed arguments: %s", got)
	}
}

func TestClaudeManualCommandsPreserveEnvironmentAndExtraFields(t *testing.T) {
	t.Parallel()
	entry := testenv.NativeFixture(`{"type":"stdio","command":"C:/space 中文/obit.exe","args":["mcp"],"env":{"Z":"a=b & %PATH% $x '","A":"first"}}`)
	want := testenv.NativeFixture(`& 'claude' 'mcp' 'add' '--scope' 'user' '-e' 'A=first' '-e' 'Z=a=b & %PATH% $x ''' '--transport' 'stdio' 'obituary' '--' 'C:/space 中文/obit.exe' 'mcp'`)
	if got := CommandText(claudeAddJSON([]byte(entry))); got != want {
		t.Fatalf("manual env syntax/quoting: %s", got)
	}
	full := strings.TrimSuffix(entry, "}") + `,"custom":42}`
	got := CommandText(claudeAddJSON([]byte(full)))
	if !strings.Contains(got, "'add-json'") || !strings.Contains(got, "PowerShell 7.3 or newer") || !strings.Contains(got, `"custom":42`) {
		t.Fatalf("full manual entry lost fields/version requirement: %s", got)
	}
}

func claudeCalls(t *testing.T, path string) [][]string {
	t.Helper()
	var calls [][]string
	for _, line := range strings.Split(strings.TrimSpace(read(t, path)), "\n") {
		var args []string
		if err := json.Unmarshal([]byte(line), &args); err != nil {
			t.Fatal(err)
		}
		calls = append(calls, args)
	}
	return calls
}

func claudeDeadlineWhenReady(t *testing.T, path string, timeout time.Duration, call int) <-chan time.Time {
	t.Helper()
	previous := claudeMCPWithTimeout
	t.Cleanup(func() { claudeMCPWithTimeout = previous })
	ctx, started := testenv.ReadyDeadline(t, path, timeout)
	claudeMCPWithTimeout = func(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
		call--
		if call == 0 {
			return ctx, func() {}
		}
		return previous(parent, d)
	}
	return started
}

func TestClaudeMCPTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	testenv.FakeClaude(t)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("OBIT_FAKE_CLAUDE_MODE", "sleep")
	old := claudeMCPTimeout
	claudeMCPTimeout = 100 * time.Millisecond
	t.Cleanup(func() { claudeMCPTimeout = old })
	plan, err := PlanMCP(Scope{Global: true}, "claude", testenv.NativeFixture("C:/tools/obit.exe"), false)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	err = Apply(plan)
	if err == nil || !strings.Contains(err.Error(), "deadline exceeded") || time.Since(started) > 3*time.Second {
		t.Fatalf("CLI timeout failed: %v", err)
	}
	if _, err := os.Stat(plan[0].Path); !os.IsNotExist(err) {
		t.Fatal("timeout wrote state")
	}
}

func TestClaudeMCPUsesOfficialCLI(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	_, log := testenv.FakeClaude(t)
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	scope := Scope{Global: true}
	path := filepath.Join(root, ".claude.json")
	writeIntegrationFile(t, path, `{"theme":"old","mcpServers":{"other":{"command":"user"}}}`)
	for i, exe := range []string{testenv.NativeFixture("C:/old/obit.exe"), testenv.NativeFixture("C:/new path/obit.exe")} {
		plan, err := PlanMCP(scope, "claude", exe, false)
		if err != nil {
			t.Fatal(err)
		}
		// Claude can save unrelated runtime state while the prompt is open.
		if i > 0 {
			current := read(t, path)
			writeIntegrationFile(t, path, strings.Replace(current, `"old"`, `"runtime"`, 1))
		}
		if err := Apply(plan); err != nil {
			t.Fatalf("CLI merge failed: %v", err)
		}
		if _, err := os.Stat(log); err != nil {
			t.Fatalf("official CLI was not invoked: %v", err)
		}
		if i > 0 && !strings.Contains(read(t, path), `"runtime"`) || !strings.Contains(read(t, path), `"other"`) {
			t.Fatal("runtime state lost")
		}
		plan, err = PlanMCP(scope, "claude", exe, false)
		if err != nil || Pending(plan) {
			t.Fatalf("CLI install not idempotent: %v %v", plan, err)
		}
	}
	plan, err := PlanMCP(scope, "claude", testenv.NativeFixture("C:/new path/obit.exe"), true)
	applyIntegration(t, plan, err)
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("official CLI was not invoked: %v", err)
	}
	var calls [][]string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var args []string
		if err := json.Unmarshal([]byte(line), &args); err != nil {
			t.Fatal(err)
		}
		calls = append(calls, args)
	}
	add := func(exe string) []string {
		return []string{"mcp", "add-json", "--scope", "user", "obituary", `{"type":"stdio","command":"` + exe + `","args":["mcp"]}`}
	}
	remove := []string{"mcp", "remove", "--scope", "user", "obituary"}
	version := []string{"--version"}
	want := [][]string{version, add(testenv.NativeFixture("C:/old/obit.exe")), version, remove, add(testenv.NativeFixture("C:/new path/obit.exe")), version, remove}
	if len(calls) != len(want) {
		t.Fatalf("official CLI arguments: %v != %v", calls, want)
	}
	for i := range calls {
		if len(calls[i]) == 6 && len(want[i]) == 6 && sameJSON([]byte(calls[i][5]), []byte(want[i][5])) {
			calls[i][5] = want[i][5]
		}
		if !reflect.DeepEqual(calls[i], want[i]) {
			t.Fatalf("official CLI arguments: %v != %v", calls, want)
		}
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Fatal("global state file was backed up/edited directly")
	}
}

func TestClaudeMCPPreservesUnownedEntries(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	_, log := testenv.FakeClaude(t)
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	path := filepath.Join(root, ".claude.json")
	original := `{"mcpServers":{"obituary":{"command":"user-tool","args":["mcp"]}}}`
	writeIntegrationFile(t, path, original)
	if _, err := PlanMCP(Scope{Global: true}, "claude", testenv.NativeFixture("C:/tools/obit.exe"), false); err == nil {
		t.Fatal("unowned registration claimed")
	}
	plan, err := PlanMCP(Scope{Global: true}, "claude", testenv.NativeFixture("C:/tools/obit.exe"), true)
	applyIntegration(t, plan, err)
	if read(t, path) != original {
		t.Fatal("unowned registration changed")
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("CLI called for unowned registration")
	}
}

func TestClaudeMCPVerifiesCLIResult(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, mode := range []string{"noop", "fail"} {
		t.Run(mode, func(t *testing.T) {
			testenv.FakeClaude(t)
			t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
			t.Setenv("OBIT_FAKE_CLAUDE_MODE", mode)
			plan, err := PlanMCP(Scope{Global: true}, "claude", testenv.NativeFixture("C:/tools/obit.exe"), false)
			if err != nil {
				t.Fatal(err)
			}
			err = Apply(plan)
			if err == nil {
				t.Fatal("unverified CLI operation reported success")
			}
			if mode == "fail" && (!strings.Contains(err.Error(), "fake stdout diagnostic") || !strings.Contains(err.Error(), "fake stderr diagnostic")) {
				t.Fatalf("CLI diagnostics lost: %v", err)
			}
		})
	}
}

func TestClaudeMCPMissingCLI(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	plan, err := PlanMCP(Scope{Global: true}, "claude", testenv.NativeFixture("C:/tools/obit.exe"), false)
	if err != nil {
		t.Fatal(err)
	}
	err = Apply(plan)
	if err == nil || !strings.Contains(err.Error(), CommandText(plan[0].Commands[0])) || !strings.Contains(err.Error(), "PowerShell") {
		t.Fatalf("missing manual command: %v", err)
	}
	if _, err := os.Stat(plan[0].Path); !os.IsNotExist(err) {
		t.Fatal("missing CLI fell back to direct write")
	}
}
