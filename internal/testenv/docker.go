package testenv

import (
	"encoding/json"
	"os"

	"github.com/woodfishhhh/obituary/internal/docker"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// Tests must never reach a real Docker. Put a refusing docker first on PATH
// before any application service opens, instead of removing directories that
// contain docker: on CI runners docker sits in System32 or /usr/bin next to
// cmd.exe, git and sh. Fixtures prepend their own executable afterwards, and
// tests that need Docker to be absent set PATH explicitly.
func isolateDocker(scratch string) error {
	dir := filepath.Join(scratch, "no-docker")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	name, script, mode := "docker", "#!/bin/sh\necho 'docker is disabled in Obituary tests' >&2\nexit 1\n", os.FileMode(0o700)
	if runtime.GOOS == "windows" {
		name, script, mode = "docker.cmd", "@echo docker is disabled in Obituary tests 1>&2\r\n@exit /b 1\r\n", 0o600
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), mode); err != nil {
		return err
	}
	return os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

var dockerImage = sync.OnceValues(func() (string, error) {
	_, source, _, _ := runtime.Caller(0)
	image := filepath.Join(fakeClaudeScratch, "fake-docker.exe")
	cmd := exec.Command("go", "build", "-o", image, filepath.Join(filepath.Dir(source), "..", "docker", "testdata", "main.go"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", &dockerBuildError{err, string(out)}
	}
	return image, nil
})

type dockerBuildError struct {
	error
	output string
}

func (e *dockerBuildError) Error() string { return e.error.Error() + ": " + e.output }

func WriteDocker(t *testing.T, path string, rows ...docker.Container) {
	t.Helper()
	var output []map[string]any
	for _, row := range rows {
		mounts := []map[string]string{}
		for _, source := range row.Binds {
			mounts = append(mounts, map[string]string{"Type": "bind", "Source": source})
		}
		output = append(output, map[string]any{"Id": row.ID, "Created": row.Created, "Name": "/" + row.Name, "State": map[string]bool{"Running": row.Running}, "Config": map[string]any{"Labels": row.Labels, "StopTimeout": row.StopTimeout}, "HostConfig": map[string]bool{"AutoRemove": row.AutoRemove}, "Mounts": mounts})
	}
	data, err := json.Marshal(output)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// DockerCommand returns the subcommand after an explicitly pinned global target.
func DockerCommand(args []string) []string {
	if len(args) > 2 && (args[0] == "--host" || args[0] == "--context") {
		return args[2:]
	}
	return args
}

func DockerCalls(t *testing.T, path string) [][]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var args []string
		if err := json.Unmarshal([]byte(line), &args); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, args)
	}
	return rows
}

// FakeDocker compiles the standalone JSON fixture once per test package.
func FakeDocker(t *testing.T) (executable, state, log string) {
	t.Helper()
	image, err := dockerImage()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	executable, state, log = filepath.Join(dir, "docker.exe"), filepath.Join(dir, "state.json"), filepath.Join(dir, "calls.jsonl")
	if runtime.GOOS != "windows" {
		executable = filepath.Join(dir, "docker")
	}
	if err := os.Link(image, executable); err != nil {
		if err := copyFakeClaude(image, executable); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("OBIT_FAKE_DOCKER_STATE", state)
	t.Setenv("OBIT_FAKE_DOCKER_LOG", log)
	t.Setenv("OBIT_FAKE_DOCKER_MODE", "")
	t.Setenv("DOCKER_CONTEXT", "")
	t.Setenv("DOCKER_HOST", "")
	return executable, state, log
}
