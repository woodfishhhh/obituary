//go:build docker_e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDockerVolumeRealEngine(t *testing.T) {
	if _, err := os.Stat(dockerBinary); err != nil {
		t.Fatal("docker is required for the docker_e2e job")
	}
	name := fmt.Sprintf("obit-e2e-%d", time.Now().UnixNano())
	runDocker(t, "volume", "rm", "-f", name)
	t.Cleanup(func() { runDockerAllowFailure(name, "volume", "rm", "-f", name) })
	runDocker(t, "volume", "create", "--", name)
	runDocker(t, "run", "--rm", "--mount", "type=volume,source="+name+",destination=/data", "busybox:1.36", "sh", "-c", "printf volume-data > /data/value")

	home := t.TempDir()
	env := append(os.Environ(), "OBIT_HOME="+home, "OBIT_CONFIG_DIR="+filepath.Join(home, "config"), "OBIT_USER_HOME="+home, "PATH=/usr/local/bin:/usr/bin:/bin")
	add := runObit(t, env, "add", "--docker-volume", name, "10m", "--json")
	var added []struct{ Item struct{ ID int64 } }
	if err := json.Unmarshal([]byte(add), &added); err != nil || len(added) != 1 || added[0].Item.ID == 0 {
		t.Fatalf("add did not return an item: %s %v", add, err)
	}
	bury := runObit(t, env, "bury", "--docker-volume", "--json", "--", name)
	var rows []struct {
		Action string
		Burial *struct {
			ID int64
		}
	}
	if err := json.Unmarshal([]byte(bury), &rows); err != nil || len(rows) != 1 || rows[0].Action != "buried" || rows[0].Burial == nil {
		t.Fatalf("bury did not report a burial: %s %v", bury, err)
	}
	if _, err := runDockerAllowFailure(name, "volume", "inspect", "--", name); err == nil {
		t.Fatal("volume still exists after burial")
	}
	resurrect := runObit(t, env, "resurrect", fmt.Sprintf("#%d", rows[0].Burial.ID), "--json")
	if !strings.Contains(resurrect, "resurrected") {
		t.Fatalf("resurrect did not complete: %s", resurrect)
	}
	got := runDocker(t, "run", "--rm", "--mount", "type=volume,source="+name+",destination=/data", "busybox:1.36", "sh", "-c", "cat /data/value")
	if got != "volume-data" {
		t.Fatalf("restored volume bytes differ: %q", got)
	}
}

func runObit(t *testing.T, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", append([]string{"run", "./cmd/obit"}, args...)...)
	cmd.Dir = filepath.Join("..", "..")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("obit %v: %v\n%s", args, err, out)
	}
	return string(bytes.TrimSpace(out))
}

func runDocker(t *testing.T, args ...string) string {
	t.Helper()
	out, err := runDockerAllowFailure("", args...)
	if err != nil {
		t.Fatalf("docker %v: %v\n%s", args, err, out)
	}
	return out
}

var dockerBinary = "/usr/bin/docker"

func runDockerAllowFailure(_ string, args ...string) (string, error) {
	cmd := exec.Command(dockerBinary, args...)
	out, err := cmd.CombinedOutput()
	return string(bytes.TrimSpace(out)), err
}
