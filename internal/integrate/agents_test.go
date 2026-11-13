package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/woodfishhhh/obituary/internal/hook"
)

// These fixtures are captured from the pre-declarative planners. Only the
// disposable root is normalized; bytes, ordering, errors and receipt phases
// remain part of the contract. No host CLI is executed and Apply is not called.
func TestAgentPlanGolden(t *testing.T) {
	if testing.Short() {
		t.Skip("complete installation plan matrix uses real isolated filesystem reads")
	}
	if runtime.GOOS != "windows" {
		t.Skip("Windows installation contract")
	}
	if len(Agents()) != 30 {
		t.Fatalf("update the explicitly reviewed inventory: got %d agents", len(Agents()))
	}
	for _, agent := range Agents() {
		t.Run(agent.Name, func(t *testing.T) {
			root := agentGoldenEnvironment(t)
			var cases []any
			for _, global := range []bool{false, true} {
				for _, uninstall := range []bool{false, true} {
					for flags := range 8 {
						for _, powershell := range []bool{false, true} {
							s := Scope{Global: global, Project: filepath.Join(root, "project"), GuardRM: flags&1 != 0, MCP: flags&2 != 0, Skill: flags&4 != 0}
							commands := ownershipCommands(agent.Name)
							commands.POSIX = "'/mnt/c/tools/obit.exe' hook " + agent.Name
							if powershell {
								commands.Native = "  & " + commands.Native + "  "
							}
							cases = append(cases, agentGoldenCase(agent, s, commands, uninstall, flags, powershell))
						}
					}
				}
			}
			data, err := json.MarshalIndent(cases, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			data = agentGoldenNormalize(data, root)
			agentGoldenCompare(t, agent.Name, append(data, '\n'))
		})
	}
}

func TestAgentPlanGoldenSmoke(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows installation contract")
	}
	for _, name := range []string{"gemini", "cursor", "copilot"} {
		t.Run(name, func(t *testing.T) {
			root := agentGoldenEnvironment(t)
			agent, _ := LookupAgent(name)
			commands := ownershipCommands(name)
			commands.POSIX = "'/mnt/c/tools/obit.exe' hook " + name
			got, err := json.Marshal(agentGoldenCase(agent, Scope{Project: filepath.Join(root, "project")}, commands, false, 0, false))
			if err != nil {
				t.Fatal(err)
			}
			golden := agentGoldenRead(t, name)
			var cases []json.RawMessage
			if err := json.Unmarshal(golden, &cases); err != nil {
				t.Fatal(err)
			}
			var want bytes.Buffer
			if err := json.Compact(&want, cases[0]); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(agentGoldenNormalize(got, root), want.Bytes()) {
				t.Fatal("default project installation changed")
			}
		})
	}
}

func agentGoldenEnvironment(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, key := range []string{
		"OBIT_HOME", "OBIT_CONFIG_DIR", "OBIT_USER_HOME", "LOCALAPPDATA", "CLAUDE_CONFIG_DIR", "CODEX_HOME",
		"QODER_CONFIG_DIR", "QODERCN_CONFIG_DIR", "VIBE_HOME", "KIMI_SHARE_DIR", "DSH_HOME", "COPILOT_HOME",
		"GEMINI_CLI_HOME", "FACTORY_HOME_OVERRIDE", "HERMES_HOME", "CLINE_DIR", "CLINE_DATA_DIR", "KILO_CONFIG_DIR",
		"OPENCODE_CONFIG_DIR", "OPENCLAW_STATE_DIR", "OPENCLAW_HOME", "PI_CODING_AGENT_DIR", "XDG_CONFIG_HOME",
		"CODEBUDDY_CONFIG_DIR", "MINIMAX_DATA_DIR", "MAVIS_DATA_DIR", "MCODE_INSTALL_DIR", "ZCODE_DATA_BASE_DIR",
	} {
		dir := filepath.Join(root, key)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv(key, dir)
	}
	if err := os.Mkdir(filepath.Join(root, "project"), 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func agentGoldenCase(agent Agent, s Scope, commands Commands, uninstall bool, flags int, powershell bool) any {
	result := map[string]any{"global": s.Global, "uninstall": uninstall, "flags": flags, "powershell": powershell}
	changes, err := agent.Plan(s, commands, uninstall)
	result["hooks"] = agentGoldenResult(changes, err)
	if flags&2 != 0 {
		var extra []Change
		var extraErr error
		if agent.PlanMCP != nil {
			extra, extraErr = agent.PlanMCP(s, commands.Executable, uninstall)
		} else {
			extraErr = fmt.Errorf("MCP planner unavailable")
		}
		result["mcp"] = agentGoldenResult(extra, extraErr)
		changes = append(changes, extra...)
	}
	if flags&4 != 0 {
		var extra []Change
		var extraErr error
		if agent.PlanSkill != nil {
			extra, extraErr = agent.PlanSkill(s, uninstall)
		} else {
			extraErr = fmt.Errorf("Skill planner unavailable")
		}
		result["skill"] = agentGoldenResult(extra, extraErr)
		changes = append(changes, extra...)
	}
	combined, composeErr := ComposeAgentChanges(changes)
	result["composed"] = agentGoldenResult(combined, composeErr)
	return result
}

func agentGoldenResult(changes []Change, err error) any {
	result := map[string]any{"changes": agentGoldenValue(reflect.ValueOf(changes))}
	if err != nil {
		result["error"] = err.Error()
		result["errorType"] = fmt.Sprintf("%T", err)
	}
	return result
}

// Reflect every plan field, including private owner intentions and next receipts.
// File identity/ancestor handles are OS-instance evidence, not serialized plans.
func agentGoldenValue(v reflect.Value) any {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return agentGoldenValue(v.Elem())
	case reflect.Struct:
		out := map[string]any{}
		for i := range v.NumField() {
			name := v.Type().Field(i).Name
			// Optional shortcut transactions have no state in agent plans.
			if name == "relocated" && v.Field(i).IsNil() {
				continue
			}
			if name != "location" && name != "identity" {
				out[name] = agentGoldenValue(v.Field(i))
			}
		}
		return out
	case reflect.Slice:
		if v.IsNil() {
			return nil
		}
		if v.Type().Elem().Kind() == reflect.Uint8 {
			data := v.Bytes()
			if utf8.Valid(data) {
				return string(data)
			}
			return map[string]string{"base64": base64.StdEncoding.EncodeToString(data)}
		}
		out := make([]any, v.Len())
		for i := range v.Len() {
			out[i] = agentGoldenValue(v.Index(i))
		}
		return out
	case reflect.String:
		return v.String()
	case reflect.Bool:
		return v.Bool()
	case reflect.Int:
		return v.Int()
	default:
		panic("unhandled plan field: " + v.Type().String())
	}
}

func agentGoldenNormalize(data []byte, root string) []byte {
	var tree any
	if err := json.Unmarshal(data, &tree); err != nil {
		panic(err)
	}
	hashes := map[string]string{}
	agentGoldenAssetHashes(tree, root, hashes)
	data = agentGoldenPaths(data, root)
	for before, after := range hashes {
		data = bytes.ReplaceAll(data, []byte(before), []byte(after))
	}
	return data
}

func agentGoldenAssetHashes(tree any, root string, hashes map[string]string) {
	switch v := tree.(type) {
	case map[string]any:
		if path, ok := v["Path"].(string); ok && strings.HasPrefix(path, root) {
			key := func(value string) string {
				return strings.Map(func(r rune) rune {
					if r >= 'A' && r <= 'Z' {
						return r + ('a' - 'A')
					}
					return r
				}, filepath.Clean(value))
			}
			hashes[fmt.Sprintf("%x", sha256.Sum256([]byte(key(path))))] = fmt.Sprintf("%x", sha256.Sum256(agentGoldenPaths([]byte(key(path)), strings.ToLower(root))))
		}
		// ManagedAsset content may include absolute runtime paths (DeepSeek).
		// Normalize its derived receipt digest, never the payload bytes.
		if v["Owner"] != nil && v["ID"] != nil {
			if content, ok := v["Content"].(string); ok {
				normalized := agentGoldenPaths([]byte(content), root)
				hashes[fmt.Sprintf("%x", sha256.Sum256([]byte(content)))] = fmt.Sprintf("%x", sha256.Sum256(normalized))
			}
		}
		for _, child := range v {
			agentGoldenAssetHashes(child, root, hashes)
		}
	case []any:
		for _, child := range v {
			agentGoldenAssetHashes(child, root, hashes)
		}
	}
}

func agentGoldenPaths(data []byte, root string) []byte {
	// Some content is JSON embedded in JSON; normalize each escaping depth.
	for _, spelling := range []string{root, filepath.ToSlash(root)} {
		for range 5 {
			data = bytes.ReplaceAll(data, []byte(spelling), []byte("<ROOT>"))
			encoded, _ := json.Marshal(spelling)
			spelling = string(encoded[1 : len(encoded)-1])
		}
	}
	return data
}

func agentGoldenCompare(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "agent-plans", name+".json.gz")
	if os.Getenv("OBIT_UPDATE_AGENT_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		var compressed bytes.Buffer
		w := gzip.NewWriter(&compressed)
		if _, err := w.Write(got); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, compressed.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want := agentGoldenRead(t, name)
	if !bytes.Equal(got, want) {
		i := 0
		for i < len(got) && i < len(want) && got[i] == want[i] {
			i++
		}
		t.Fatalf("installation plan changed at byte %d: got %q; want %q", i, got[i:min(i+180, len(got))], want[i:min(i+180, len(want))])
	}
}

func agentGoldenRead(t *testing.T, name string) []byte {
	t.Helper()
	compressed, err := os.ReadFile(filepath.Join("testdata", "agent-plans", name+".json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	want, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return want
}

func TestAgentCoverageDeclarations(t *testing.T) {
	t.Parallel()
	for _, host := range hook.Agents() {
		if _, ok := LookupAgent(host.Name); !ok {
			t.Errorf("%s has no integration declaration for agent coverage", host.Name)
		}
	}
	for _, agent := range Agents() {
		t.Run(agent.Name, func(t *testing.T) {
			switch agent.Method {
			case NativeHooks, Plugin, RulesAndMCP:
			default:
				t.Errorf("%s must declare its integration method", agent.Name)
			}
			c := agent.Capabilities
			for name, support := range map[string]hook.Support{
				"write observation": c.WriteObservation, "session start": c.SessionStart,
				"stop": c.Stop, "session end": c.SessionEnd, "context": c.Context,
				"delete guard": c.DeleteGuard, "MCP": c.MCP, "skill": c.Skill,
			} {
				if support.Level != hook.Supported && strings.TrimSpace(support.Detail) == "" {
					t.Errorf("%s %s must explain limited or unsupported coverage", agent.Name, name)
				}
			}
		})
	}
}

func TestAgentRegistryPlansExistingAgents(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	commands := Commands{Native: testenv.NativeFixture(`"C:/tools/obit.exe" hook claude`), POSIX: "'/mnt/c/tools/obit.exe' hook codex", Executable: testenv.NativeFixture("C:/tools/obit.exe")}
	for _, name := range []string{"claude", "codex"} {
		t.Run(name, func(t *testing.T) {
			agent, ok := LookupAgent(name)
			host, _ := hook.Lookup(name)
			if !ok || agent.Capabilities != host.Capabilities || agent.PlanMCP == nil || agent.PlanSkill == nil {
				t.Fatalf("incomplete registry: %+v", agent)
			}
			for _, global := range []bool{false, true} {
				for _, uninstall := range []bool{false, true} {
					scope := Scope{Global: global, Project: t.TempDir(), GuardRM: true}
					got, err := PlanAgent(name, scope, commands, uninstall)
					if err != nil {
						t.Fatal(err)
					}
					var want []Change
					if name == "claude" {
						want, err = PlanClaude(scope, commands.Native, uninstall)
					} else {
						want, err = PlanCodex(scope, commands.POSIX, commands.Native, uninstall)
					}
					if err != nil || !reflect.DeepEqual(agentGoldenValue(reflect.ValueOf(got)), agentGoldenValue(reflect.ValueOf(want))) {
						t.Fatalf("planner changed behavior: %+v %+v %v", got, want, err)
					}
				}
			}
		})
	}
	if _, err := PlanAgent("unknown", Scope{}, Commands{}, false); err == nil {
		t.Fatal("unknown agent accepted")
	}
	all := Agents()
	for i := 1; i < len(all); i++ {
		if all[i-1].Name >= all[i].Name {
			t.Fatal("unordered registry")
		}
	}
	all[0].Name = "changed"
	if Agents()[0].Name == "changed" {
		t.Fatal("caller changed registry")
	}
}

func TestAgentRegistryRejectsInvalidEntries(t *testing.T) {
	existing, _ := LookupAgent("claude")
	for _, agent := range []Agent{{}, {Name: "missing"}, existing} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("invalid registration accepted: %+v", agent)
				}
			}()
			RegisterAgent(agent)
		}()
	}
}
