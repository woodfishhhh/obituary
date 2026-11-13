package testenv_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGoBuildDirectoriesAreExplicit(t *testing.T) {
	for _, name := range []string{"GOMODCACHE", "GOCACHE", "GOPATH"} {
		if os.Getenv(name) == "" {
			t.Errorf("%s was left dependent on the isolated home", name)
		}
	}
}

func TestGoTelemetryOffInIsolatedConfig(t *testing.T) {
	config, err := os.UserConfigDir()
	if err != nil || runtime.GOOS == "windows" {
		t.Skip("the user config directory is not isolated on this platform")
	}
	if data, err := os.ReadFile(filepath.Join(config, "go", "telemetry", "mode")); err != nil || string(data) != "off" {
		t.Fatalf("Go telemetry may write into the isolated home during cleanup: %q %v", data, err)
	}
	if out, err := exec.Command("go", "env", "GOTELEMETRY").Output(); err != nil || strings.TrimSpace(string(out)) != "off" {
		t.Fatalf("go command still sees telemetry enabled: %q %v", out, err)
	}
}

func TestGoBuildDirectoriesSurviveHomeIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("starts Go and an isolated test subprocess")
	}
	for _, configured := range []bool{false, true} {
		t.Run(map[bool]string{false: "defaults", true: "goenv-file"}[configured], func(t *testing.T) {
			home := t.TempDir()
			env := goBuildEnvironment(home)
			if configured {
				config := filepath.Join(home, "goenv")
				text := ""
				for _, name := range []string{"GOMODCACHE", "GOCACHE", "GOPATH"} {
					text += name + "=" + filepath.Join(home, name) + "\n"
				}
				if err := os.WriteFile(config, []byte(text), 0o600); err != nil {
					t.Fatal(err)
				}
				env = append(env, "GOENV="+config)
			}
			want := readGoBuildEnvironment(t, env)
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(exe, "-test.run=^TestGoBuildDirectoriesChild$")
			cmd.Env = append(env, "OBIT_TEST_GO_ENV="+string(data))
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("isolated Go directories changed: %s %v", out, err)
			}
		})
	}
}

func TestGoBuildDirectoriesChild(t *testing.T) {
	if testing.Short() || os.Getenv("OBIT_TEST_GO_ENV") == "" {
		t.Skip("subprocess assertion")
	}
	var want map[string]string
	if err := json.Unmarshal([]byte(os.Getenv("OBIT_TEST_GO_ENV")), &want); err != nil {
		t.Fatal(err)
	}
	got := readGoBuildEnvironment(t, os.Environ())
	for name, path := range want {
		if explicit := os.Getenv(name); explicit != path || got[name] != path {
			t.Errorf("%s: explicit=%q effective=%q, want %q", name, explicit, got[name], path)
		}
	}
}

func goBuildEnvironment(home string) []string {
	var env []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(name) {
		case "GOMODCACHE", "GOCACHE", "GOPATH", "HOME", "USERPROFILE", "LOCALAPPDATA", "XDG_CACHE_HOME", "XDG_CONFIG_HOME", "GOENV":
			continue
		}
		env = append(env, entry)
	}
	return append(env, "HOME="+home, "USERPROFILE="+home, "LOCALAPPDATA="+home, "GOENV=off")
}

func readGoBuildEnvironment(t *testing.T, env []string) map[string]string {
	t.Helper()
	cmd := exec.Command("go", "env", "-json", "GOMODCACHE", "GOCACHE", "GOPATH")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go env: %s %v", out, err)
	}
	var paths map[string]string
	if err := json.Unmarshal(out, &paths); err != nil {
		t.Fatal(err)
	}
	return paths
}
