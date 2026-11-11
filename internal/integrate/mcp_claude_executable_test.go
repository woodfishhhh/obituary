//go:build windows

package integrate

import (
	"github.com/woodfishhhh/obituary/internal/testenv"
	"github.com/woodfishhhh/obituary/internal/winfs"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestClaudeMCPVolumeRootAliases(t *testing.T) {
	for _, mode := range []string{"local", "unc", "extended-unc"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			cwd := filepath.Join(root, "work")
			if err := os.Mkdir(cwd, 0o700); err != nil {
				t.Fatal(err)
			}
			// No Git root: discovery must reach the share root without crossing it.
			external := filepath.Join(t.TempDir(), "claude.exe")
			writeIntegrationFile(t, external, "native candidate; never executed")
			inside := filepath.Join(root, "tools", "claude.exe")
			writeIntegrationFile(t, inside, "repository candidate; never executed")
			alias := func(path string) string {
				if mode == "local" {
					return path
				}
				volume := filepath.VolumeName(path)
				path = `\\localhost\` + strings.TrimSuffix(volume, ":") + "$" + path[len(volume):]
				if mode == "extended-unc" {
					path = `\\?\UNC\` + path[2:]
				}
				return path
			}
			if _, err := os.Stat(alias(cwd)); err != nil {
				t.Skipf("administrative share unavailable: %v", err)
			}
			t.Chdir(alias(cwd))
			if _, err := checkedClaudeExecutable(external); err != nil {
				t.Fatalf("external executable rejected across volume root: %v", err)
			}
			if _, err := checkedClaudeExecutable(alias(external)); err != nil {
				t.Fatalf("external UNC candidate rejected: %v", err)
			}
			id, err := winfs.IdentifyFollowing(cwd)
			if err != nil {
				t.Fatal(err)
			}
			if err := checkClaudeAncestors(alias(external), []claudeExecutableRoot{{path: cwd, id: id, cwd: true}}); err != nil {
				t.Fatalf("external ancestor traversal crossed volume root: %v", err)
			}
			if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
				t.Fatal(err)
			}
			if _, err := checkedClaudeExecutable(alias(inside)); err == nil || !strings.Contains(err.Error(), "inside the current directory or Git project") {
				t.Fatalf("repository executable accepted across volume alias: %v", err)
			}
			if err := os.Link(external, filepath.Join(root, "linked-claude.exe")); err != nil {
				t.Fatal(err)
			}
			if _, err := checkedClaudeExecutable(alias(external)); err == nil {
				t.Fatal("unverifiable remote hardlinks accepted")
			}
			underCWD := filepath.Join(cwd, "claude.exe")
			writeIntegrationFile(t, underCWD, "cwd candidate")
			if _, err := checkedClaudeExecutable(alias(underCWD)); err == nil || !strings.Contains(err.Error(), "run obit init from a directory that does not contain your Claude installation") {
				t.Fatalf("cwd refusal lacks relocation guidance: %v", err)
			}
		})
	}
}

func TestClaudeMCPRejectsExecutableAliases(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, mode := range []string{"cwd-junction", "unc", "extended", "short-name", "trailing-dot", "project-junction", "project-junction-trailing-dot", "external-junction", "hardlink", "file-symlink"} {
		t.Run(mode, func(t *testing.T) {
			fake, log := testenv.FakeClaude(t)
			t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
			root := filepath.Join(t.TempDir(), "repository long name")
			cwd, tools := filepath.Join(root, "work"), filepath.Join(root, "tools")
			for _, dir := range []string{cwd, tools} {
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			testenv.InitRepo(t, root)
			// Use an independent image: other fixture hard links must not mask
			// the identity assertion with a separate repository-owned name.
			exe := filepath.Join(tools, "claude.exe")
			if strings.HasPrefix(mode, "project-junction") {
				external := t.TempDir()
				exe = filepath.Join(external, "claude.exe")
				tools = filepath.Join(root, "install")
				makeClaudeJunction(t, tools, external)
			}
			data, err := os.ReadFile(fake)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(exe, data, 0o700); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "cwd-junction":
				alias := filepath.Join(t.TempDir(), "alias")
				makeClaudeJunction(t, alias, cwd)
				cwd = alias
			case "unc":
				tools = `\\localhost\` + strings.TrimSuffix(filepath.VolumeName(tools), ":") + "$" + tools[len(filepath.VolumeName(tools)):]
				if _, err := os.Stat(filepath.Join(tools, "claude.exe")); err != nil {
					t.Skipf("administrative share unavailable: %v", err)
				}
			case "extended":
				tools = `\\?\` + tools
			case "short-name":
				p, err := windows.UTF16PtrFromString(tools)
				if err != nil {
					t.Fatal(err)
				}
				buf := make([]uint16, 32768)
				n, err := windows.GetShortPathName(p, &buf[0], uint32(len(buf)))
				if err != nil {
					t.Fatal(err)
				}
				short := windows.UTF16ToString(buf[:n])
				if strings.EqualFold(short, tools) {
					t.Skip("volume has no 8.3 name for fixture")
				}
				tools = short
			case "trailing-dot", "project-junction-trailing-dot":
				tools = root + `.\` + filepath.Base(tools)
			case "external-junction":
				alias := filepath.Join(t.TempDir(), "alias")
				makeClaudeJunction(t, alias, tools)
				tools = alias
			case "hardlink":
				tools = t.TempDir()
				if err := os.Link(exe, filepath.Join(tools, "claude.exe")); err != nil {
					t.Fatal(err)
				}
			case "file-symlink":
				tools = t.TempDir()
				if err := os.Symlink(exe, filepath.Join(tools, "claude.exe")); err != nil {
					t.Skipf("file symlink unavailable: %v", err)
				}
			}
			t.Chdir(cwd)
			t.Setenv("PATH", tools)
			plan, err := PlanMCP(Scope{Global: true}, "claude", "C:/tools/obit.exe", false)
			if err != nil {
				t.Fatal(err)
			}
			err = Apply(plan)
			if _, statErr := os.Stat(log); !os.IsNotExist(statErr) {
				t.Fatalf("aliased repository executable was started: %s (%v)", mode, err)
			}
			if err == nil || !strings.Contains(err.Error(), "refus") || !strings.Contains(err.Error(), "PowerShell") {
				t.Fatalf("alias refusal lacks reason and manual command: %s %v", mode, err)
			}
		})
	}
}

func makeClaudeJunction(t *testing.T, link, target string) {
	t.Helper()
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Fatalf("junction: %v %s", err, out)
	}
}

func TestClaudeMCPAcceptsExternalHardlinks(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	fake, log := testenv.FakeClaude(t)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	dir := t.TempDir()
	exe := filepath.Join(dir, "claude.exe")
	data, err := os.ReadFile(fake)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, data, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(exe, filepath.Join(t.TempDir(), "claude.exe")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	plan, err := PlanMCP(Scope{Global: true}, "claude", "C:/tools/obit.exe", false)
	applyIntegration(t, plan, err)
	if calls := claudeCalls(t, log); len(calls) != 2 {
		t.Fatalf("external hardlinked executable was not accepted: %v", calls)
	}
}

func TestClaudeMCPRejectsUnverifiableHardlinkNames(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	_, log := testenv.FakeClaude(t)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	dir := t.TempDir()
	testenv.FakeClaudeAt(t, filepath.Join(dir, "claude.exe"))
	original, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	before, _, err := original.DACL()
	if err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sid := user.User.Sid.String()
	// Deny attributes on this directory only, retaining traversal to the exe.
	sd, err := windows.SecurityDescriptorFromString("D:(D;;0x80;;;" + sid + ")(A;;FA;;;" + sid + ")")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, before, nil); err != nil {
			t.Error(err)
		}
	})
	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	plan, err := PlanMCP(Scope{Global: true}, "claude", "C:/tools/obit.exe", false)
	if err != nil {
		t.Fatal(err)
	}
	err = Apply(plan)
	if err == nil || !strings.Contains(err.Error(), "hard-link names") || !strings.Contains(err.Error(), "PowerShell") {
		t.Fatalf("unverifiable hard links lack refusal and manual command: %v", err)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatalf("Claude started without hard-link enumeration: %v", err)
	}
}

func TestClaudeMCPNativeNPMReturnsResolvedExecutable(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	_, log := testenv.FakeClaude(t)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	npm := t.TempDir()
	writeIntegrationFile(t, filepath.Join(npm, "claude.cmd"), "never executed")
	native := filepath.Join(npm, "node_modules", "@anthropic-ai", "claude-code", "bin", "claude.exe")
	testenv.FakeClaudeAt(t, native)
	alias := filepath.Join(t.TempDir(), "npm-alias")
	makeClaudeJunction(t, alias, npm)
	t.Setenv("PATH", alias)
	got, err := resolveClaudeExecutable()
	if err != nil || got != native {
		t.Fatalf("npm did not retain verified executable: got=%s want=%s err=%v", got, native, err)
	}
	plan, err := PlanMCP(Scope{Global: true}, "claude", "C:/tools/obit.exe", false)
	applyIntegration(t, plan, err)
	if len(claudeCalls(t, log)) != 2 {
		t.Fatal("resolved npm executable was not invoked")
	}
}

func TestClaudeMCPRejectsUnverifiableExecutable(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	_, log := testenv.FakeClaude(t)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	old := claudeLookPath
	t.Cleanup(func() { claudeLookPath = old })
	// A disappearing lookup must never fall through to an installed client.
	t.Setenv("PATH", t.TempDir())
	claudeLookPath = func(string) (string, error) { return filepath.Join(t.TempDir(), "missing.exe"), nil }
	plan, err := PlanMCP(Scope{Global: true}, "claude", "C:/tools/obit.exe", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err == nil || !strings.Contains(err.Error(), "PowerShell") {
		t.Fatalf("unverifiable executable accepted: %v", err)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatalf("unverifiable executable was started: %v", err)
	}
}

func TestClaudeMCPRejectsUntrustedExecutablePaths(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, mode := range []string{"dot", "relative", "empty", "absolute-project", "native-project", "native-cwd"} {
		t.Run(mode, func(t *testing.T) {
			_, log := testenv.FakeClaude(t)
			t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
			root := t.TempDir()
			testenv.InitRepo(t, root)
			cwd := filepath.Join(root, "work")
			if err := os.Mkdir(cwd, 0o700); err != nil {
				t.Fatal(err)
			}
			t.Chdir(cwd)
			// Exercise Go's default Windows dot lookup independently of host policy.
			t.Setenv("NoDefaultCurrentDirectoryInExePath", "")
			if err := os.Unsetenv("NoDefaultCurrentDirectoryInExePath"); err != nil {
				t.Fatal(err)
			}
			t.Setenv("GODEBUG", "execerrdot=1")
			dir, path := cwd, "."
			switch mode {
			case "relative":
				dir, path = filepath.Join(cwd, "tools"), "tools"
			case "empty":
				path = string(os.PathListSeparator)
			case "absolute-project", "native-project":
				dir = filepath.Join(root, "tools")
				path = dir
			case "native-cwd":
				path = t.TempDir()
			}
			if strings.HasPrefix(mode, "native-") {
				testenv.FakeClaudeAt(t, filepath.Join(dir, "claude.exe"))
			} else {
				writeIntegrationFile(t, filepath.Join(dir, "claude.cmd"), "must not run")
				testenv.FakeClaudeAt(t, filepath.Join(dir, "node_modules", "@anthropic-ai", "claude-code", "bin", "claude.exe"))
			}
			t.Setenv("PATH", path)
			plan, err := PlanMCP(Scope{Global: true}, "claude", "C:/tools/obit.exe", false)
			if err != nil {
				t.Fatal(err)
			}
			err = Apply(plan)
			if _, statErr := os.Stat(log); !os.IsNotExist(statErr) {
				t.Fatalf("untrusted Claude executable was started: %s (%v)", mode, err)
			}
			if err == nil || !strings.Contains(err.Error(), "refus") || !strings.Contains(err.Error(), "PowerShell") {
				t.Fatalf("unsafe path lacks reason and manual command: %v", err)
			}
		})
	}
}

func TestClaudeMCPRequiresLockingVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, version := range []string{"2.1.317 (Claude Code)", "garbage", "", "2.1.318-beta", "2.1.318 (Claude Code)", "2.2.0", "3.0.0"} {
		t.Run(version, func(t *testing.T) {
			_, log := testenv.FakeClaude(t)
			t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
			t.Setenv("OBIT_FAKE_CLAUDE_VERSION", version)
			plan, err := PlanMCP(Scope{Global: true}, "claude", "C:/tools/obit.exe", false)
			if err != nil {
				t.Fatal(err)
			}
			err = Apply(plan)
			allowed := version == "2.1.318 (Claude Code)" || version == "2.2.0" || version == "3.0.0"
			calls := claudeCalls(t, log)
			if allowed {
				if err != nil || len(calls) != 2 || !reflect.DeepEqual(calls[0], []string{"--version"}) {
					t.Fatalf("supported version rejected: %v %v", calls, err)
				}
			} else if err == nil || len(calls) != 1 || !reflect.DeepEqual(calls[0], []string{"--version"}) || !strings.Contains(err.Error(), "2.1.318") || !strings.Contains(err.Error(), "PowerShell") {
				t.Fatalf("unsafe Claude version executed mutation: %q %v %v", version, calls, err)
			}
		})
	}
}

func TestClaudeMCPRejectsProjectExecutableJunction(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	_, log := testenv.FakeClaude(t)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	root, external := t.TempDir(), t.TempDir()
	testenv.InitRepo(t, root)
	cwd, link := filepath.Join(root, "work"), filepath.Join(root, "tools")
	if err := os.Mkdir(cwd, 0o700); err != nil {
		t.Fatal(err)
	}
	writeIntegrationFile(t, filepath.Join(external, "claude.cmd"), "must not run")
	testenv.FakeClaudeAt(t, filepath.Join(external, "node_modules", "@anthropic-ai", "claude-code", "bin", "claude.exe"))
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, external).CombinedOutput(); err != nil {
		t.Fatalf("junction: %v %s", err, out)
	}
	t.Chdir(cwd)
	t.Setenv("PATH", link)
	plan, err := PlanMCP(Scope{Global: true}, "claude", "C:/tools/obit.exe", false)
	if err != nil {
		t.Fatal(err)
	}
	err = Apply(plan)
	if _, statErr := os.Stat(log); !os.IsNotExist(statErr) {
		t.Fatalf("project junction executable was started: %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "refusing") {
		t.Fatalf("project junction not refused: %v", err)
	}
}

func TestClaudeMCPNativeNPMExecutable(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, shim := range []string{"claude.cmd", "claude", "claude.ps1"} {
		t.Run(shim, func(t *testing.T) {
			_, log := testenv.FakeClaude(t)
			t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
			npm := t.TempDir()
			writeIntegrationFile(t, filepath.Join(npm, shim), "this shim must never execute")
			native := filepath.Join(npm, "node_modules", "@anthropic-ai", "claude-code", "bin", "claude.exe")
			testenv.FakeClaudeAt(t, native)
			t.Setenv("PATH", npm)
			resolved, resolveErr := resolveClaudeExecutable()
			if resolveErr != nil || resolved != native {
				t.Fatalf("npm resolver returned wrapper instead of native exe: %s %v", resolved, resolveErr)
			}
			plan, err := PlanMCP(Scope{Global: true}, "claude", "C:/a %USERNAME% $cache ' &/obit.exe", false)
			if err != nil {
				t.Fatal(err)
			}
			if err := Apply(plan); err != nil {
				t.Fatalf("npm native executable not used: %v", err)
			}
			if len(claudeCalls(t, log)) != 2 {
				t.Fatal("npm native executable not invoked")
			}
		})
	}
}

func TestClaudeMCPNativeExecutablePriority(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	_, log := testenv.FakeClaude(t)
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	npm := t.TempDir()
	writeIntegrationFile(t, filepath.Join(npm, "claude.cmd"), "invalid wrapper")
	t.Setenv("PATH", npm+string(os.PathListSeparator)+filepath.Dir(log))
	plan, err := PlanMCP(Scope{Global: true}, "claude", "C:/tools/obit.exe", false)
	applyIntegration(t, plan, err)
}

func TestClaudeMCPRejectsWrapperWithoutNativeFile(t *testing.T) {
	for _, directory := range []bool{false, true} {
		t.Run(strconv.FormatBool(directory), func(t *testing.T) {
			npm := t.TempDir()
			t.Setenv("PATH", npm)
			t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
			writeIntegrationFile(t, filepath.Join(npm, "claude.cmd"), "must not run")
			if directory {
				if err := os.MkdirAll(filepath.Join(npm, "node_modules", "@anthropic-ai", "claude-code", "bin", "claude.exe"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			plan, err := PlanMCP(Scope{Global: true}, "claude", "C:/tools/obit.exe", false)
			if err != nil {
				t.Fatal(err)
			}
			if err := Apply(plan); err == nil || !strings.Contains(err.Error(), "PowerShell") {
				t.Fatalf("non-native CLI accepted: %v", err)
			}
		})
	}
}
