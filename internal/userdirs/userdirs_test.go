package userdirs

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWinGetPackagesUsesLocalAppData(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOCALAPPDATA", base)
	t.Setenv("OBIT_HOME", t.TempDir())
	got, err := WinGetPackages()
	want := filepath.Join(base, "Microsoft", "WinGet", "Packages")
	if err != nil || got != want {
		t.Fatalf("WinGet packages: %q %v", got, err)
	}
}

func TestRegistryLocksIndependentOfLedger(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOCALAPPDATA", base)
	t.Setenv("OBIT_HOME", t.TempDir())
	want := filepath.Join(base, "obituary", "registry-locks")
	for range 2 {
		got, err := RegistryLocks()
		if err != nil || got != want {
			t.Fatalf("registry lock scope: %q %v", got, err)
		}
		t.Setenv("OBIT_HOME", t.TempDir())
	}
}

func TestDataNormalizesExtendedHome(t *testing.T) {
	for _, home := range []string{filepath.Join(t.TempDir(), "home"), `\\localhost\C$\obit-home`} {
		t.Setenv("OBIT_HOME", home)
		plain, err := Data()
		if err != nil {
			t.Fatal(err)
		}
		verb := `\\?\` + home
		if home[:2] == `\\` {
			verb = `\\?\UNC\` + home[2:]
		}
		t.Setenv("OBIT_HOME", verb)
		normalized, err := Data()
		if err != nil || normalized != plain {
			t.Fatalf("same data directory retained different spellings: %q %q %v", plain, normalized, err)
		}
	}
}

func TestDirectoryGettersRejectUnstablePrefixes(t *testing.T) {
	for _, tc := range []struct {
		env string
		get func() (string, error)
	}{
		{"OBIT_HOME", Data}, {"OBIT_CONFIG_DIR", Config}, {"OBIT_USER_HOME", Home},
		{"CLAUDE_CONFIG_DIR", ClaudeDir}, {"CODEX_HOME", CodexDir}, {"CLAUDE_CONFIG_DIR", ClaudeMCPFile},
	} {
		t.Run(tc.env, func(t *testing.T) {
			for _, tail := range []string{"config.", "config \\child"} {
				t.Setenv(tc.env, `\\?\`+filepath.Join(t.TempDir(), tail))
				if path, err := tc.get(); err == nil {
					t.Fatalf("directory getter accepted unstable spelling: %s %q", tc.env, path)
				}
			}
			plain := filepath.Join(t.TempDir(), "normal")
			t.Setenv(tc.env, `\\?\`+plain)
			path, err := tc.get()
			if err != nil || path != plain && path != filepath.Join(plain, ".claude.json") {
				t.Fatalf("valid extended directory: %q %v", path, err)
			}
		})
	}
}

func TestAgentExtensionLocations(t *testing.T) {
	home, custom := t.TempDir(), t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	if path, err := ClaudeMCPFile(); err != nil || path != filepath.Join(home, ".claude.json") {
		t.Fatalf("Claude default: %s %v", path, err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", custom)
	if path, err := ClaudeMCPFile(); err != nil || path != filepath.Join(custom, ".claude.json") {
		t.Fatalf("Claude override: %s %v", path, err)
	}
	t.Setenv("CODEX_HOME", custom)
	if path, err := CodexSkillsDir(); err != nil || path != filepath.Join(home, ".agents", "skills") {
		t.Fatalf("shared Codex skills must follow user home: %s %v", path, err)
	}
}

func TestOnlyUserDirsResolvesUserLocations(t *testing.T) {
	t.Parallel()
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(filepath.Dir(source)))
	var reads []directoryRead
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			// bin holds ignored build/test artifacts, not repository source;
			// testdata holds fixture programs that run with explicit test homes.
			if rel == ".git" || rel == "bin" || rel == "internal/userdirs" || entry.Name() == "testdata" || entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, variable := range directoryReads(file) {
			reads = append(reads, directoryRead{rel, variable})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if violations := directoryViolations(reads, directoryExemptions); len(violations) != 0 {
		t.Fatal(strings.Join(violations, "\n"))
	}
}

func TestDirectoryReadScanner(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ code, want string }{
		{`os.UserHomeDir()`, "os.UserHomeDir"},
		{`os.UserConfigDir()`, "os.UserConfigDir"},
		{`os.UserCacheDir()`, "os.UserCacheDir"},
		{`os.LookupEnv("HOME")`, "HOME"},
		{`env := os.Getenv; env("OBIT_HOME")`, "OBIT_HOME"},
		{`const name = "CODEX_HOME"; os.Getenv(name)`, "CODEX_HOME"},
		{`const key = "LOCAL" + "APPDATA"; os.Getenv(key)`, "LOCALAPPDATA"},
		{`const key = ("LOCAL" + ("APP" + "DATA")); os.Getenv((key))`, "LOCALAPPDATA"},
		{`os.LookupEnv("LOCAL" + "" + "APPDATA")`, "LOCALAPPDATA"},
		{`prefix := getPrefix(); os.Getenv(prefix + "HOME")`, ""},
		{`os.Setenv("OBIT_HOME", "temporary")`, ""},
		{`_ = "os.Getenv(\"HOME\")" // os.UserHomeDir()`, ""},
	} {
		t.Run(tc.code, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture\nimport os \"os\"\nfunc f() {\n"+tc.code+"\n}", 0)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(directoryReads(file), ","); got != tc.want {
				t.Fatalf("reads %q, want %q", got, tc.want)
			}
		})
	}
	if got := directoryViolations(nil, map[directoryRead]string{{"old.go", "HOME"}: "removed read"}); len(got) != 1 || !strings.Contains(got[0], "unused exemption") {
		t.Fatalf("stale exemption accepted: %v", got)
	}
}

func TestAgentSkillsDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("CODEX_HOME", t.TempDir())
	got, err := AgentSkillsDir()
	if err != nil || got != filepath.Join(home, ".agents", "skills") {
		t.Fatalf("shared skills: %q %v", got, err)
	}
	t.Setenv("OBIT_USER_HOME", home+":stream")
	if _, err := AgentSkillsDir(); err == nil {
		t.Fatal("accepted unstable home")
	}
}
