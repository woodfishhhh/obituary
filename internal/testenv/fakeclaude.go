package testenv

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"
)

var fakeClaudeScratch string // set by Main; kept until all tests have finished
var fakeClaudeImage = sync.OnceValues(func() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	image := filepath.Join(fakeClaudeScratch, "fake-claude.exe")
	return image, copyFakeClaude(self, image)
})

// FakeClaude reuses TestMain's binary so tests can exercise client commands
// without invoking a compiler or an installed client.
func FakeClaude(t *testing.T) (executable, log string) {
	t.Helper()
	dir := t.TempDir()
	executable, log = filepath.Join(dir, "claude.exe"), filepath.Join(dir, "calls.jsonl")
	FakeClaudeAt(t, executable)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("OBIT_TEST_FAKE_CLAUDE", "1")
	t.Setenv("OBIT_FAKE_CLAUDE_LOG", log)
	t.Setenv("OBIT_FAKE_CLAUDE_MODE", "")
	t.Setenv("OBIT_FAKE_CLAUDE_VERSION", "2.1.318 (Claude Code)")
	return executable, log
}

// FakeClaudeAt also supports npm's nested native executable layout.
func FakeClaudeAt(t *testing.T, path string) {
	t.Helper()
	image, err := fakeClaudeImage()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	// Linking the running parent image would prevent t.TempDir cleanup. Link
	// a single, separate copy instead; each child is waited for before cleanup.
	if err := os.Link(image, path); err == nil {
		return
	}
	if err := copyFakeClaude(image, path); err != nil {
		t.Fatal(err)
	}
}

func copyFakeClaude(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, src)
	closeErr := dst.Close()
	if err != nil || closeErr != nil {
		return fmt.Errorf("copy fake claude: %w", errors.Join(err, closeErr))
	}
	return nil
}

func runFakeClaude() error {
	args := os.Args[1:]
	if slices.Equal(args, []string{"--fake-child"}) {
		// Bounded even under a broken process-tree runner; the test also reaps it.
		time.Sleep(8 * time.Second)
		return nil
	}
	f, err := os.OpenFile(os.Getenv("OBIT_FAKE_CLAUDE_LOG"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	err = json.NewEncoder(f).Encode(args)
	f.Close()
	if err != nil {
		return err
	}
	if slices.Equal(args, []string{"--version"}) {
		fmt.Println(os.Getenv("OBIT_FAKE_CLAUDE_VERSION"))
		return nil
	}
	mode := os.Getenv("OBIT_FAKE_CLAUDE_MODE")
	switch mode {
	case "fail":
		fmt.Println("fake stdout diagnostic")
		return fmt.Errorf("fake stderr diagnostic")
	case "noop":
		return nil
	case "sleep":
		time.Sleep(8 * time.Second)
		return nil
	case "flood":
		fmt.Fprintln(os.Stdout, "flood-prefix")
		chunk := bytes.Repeat([]byte("x"), 32<<10)
		for range 1024 {
			if _, err := os.Stdout.Write(chunk); err != nil {
				return err
			}
			if _, err := os.Stderr.Write(chunk); err != nil {
				return err
			}
		}
		return fmt.Errorf("flood-suffix")
	case "tree", "tree-exit":
		self, err := os.Executable()
		if err != nil {
			return err
		}
		child := exec.Command(self, "--fake-child")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		quietChild(child)
		if err := child.Start(); err != nil {
			return err
		}
		if err := os.WriteFile(os.Getenv("OBIT_FAKE_CLAUDE_CHILD"), []byte(fmt.Sprint(child.Process.Pid)), 0o600); err != nil {
			return err
		}
		if mode == "tree-exit" {
			return child.Process.Release()
		}
		defer child.Wait()
		time.Sleep(8 * time.Second)
		return nil
	}
	// This fake only accepts an explicitly redirected test configuration. It
	// cannot import userdirs: that package's TestMain imports testenv itself.
	dir := os.Getenv("CLAUDE_CONFIG_DIR")
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("fake Claude requires an absolute CLAUDE_CONFIG_DIR")
	}
	path := filepath.Join(dir, ".claude.json")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	root := map[string]any{}
	if len(data) > 0 {
		if err := json.Unmarshal(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), &root); err != nil {
			return err
		}
	}
	servers, _ := root["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	switch {
	case len(args) == 6 && slices.Equal(args[:5], []string{"mcp", "add-json", "--scope", "user", "obituary"}):
		if _, exists := servers["obituary"]; exists {
			return fmt.Errorf("MCP server obituary already exists in user config")
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(args[5]), &entry); err != nil {
			return err
		}
		if mode == "fail-add" || (mode == "fail-refresh" || mode == "restore-committed-error") && entry["command"] != os.Getenv("OBIT_FAKE_CLAUDE_OLD_COMMAND") {
			return fmt.Errorf("fake add failure")
		}
		if mode == "drop-env" {
			delete(entry, "env")
		}
		if mode == "concurrent-add" {
			entry = map[string]any{"command": "user-tool", "args": []string{}}
		}
		servers["obituary"] = entry
	case len(args) == 10 && slices.Equal(args[:8], []string{"mcp", "add", "--scope", "user", "--transport", "stdio", "obituary", "--"}) && args[9] == "mcp":
		// Kept for the compiled mutation that restores the old lossy command.
		if mode == "fail-add" || mode == "fail-refresh" && args[8] != os.Getenv("OBIT_FAKE_CLAUDE_OLD_COMMAND") {
			return fmt.Errorf("fake add failure")
		}
		servers["obituary"] = map[string]any{"type": "stdio", "command": args[8], "args": []string{"mcp"}}
	case slices.Equal(args, []string{"mcp", "remove", "--scope", "user", "obituary"}):
		if _, exists := servers["obituary"]; !exists {
			return fmt.Errorf("No user-scoped MCP server found with name: obituary") //nolint:staticcheck // ST1005: the fake CLI reproduces Claude's diagnostic for adapter tests.
		}
		delete(servers, "obituary")
		if mode == "remove-new" || mode == "remove-new-error" {
			var entry map[string]any
			if err := json.Unmarshal([]byte(os.Getenv("OBIT_FAKE_CLAUDE_NEW_ENTRY")), &entry); err != nil {
				return err
			}
			servers["obituary"] = entry
		}
	default:
		return fmt.Errorf("unexpected arguments: %q", args)
	}
	root["mcpServers"] = servers
	data, err = json.Marshal(root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	if args[1] == "remove" {
		switch mode {
		case "remove-committed-error", "remove-new-error":
			return fmt.Errorf("remove committed then failed")
		case "remove-committed-timeout":
			if err := os.WriteFile(os.Getenv("OBIT_FAKE_CLAUDE_READY"), []byte("ready"), 0o600); err != nil {
				return err
			}
			time.Sleep(8 * time.Second)
		case "remove-committed-waitdelay":
			self, err := os.Executable()
			if err != nil {
				return err
			}
			child := exec.Command(self, "--fake-child")
			child.Stdout, child.Stderr = os.Stdout, os.Stderr
			quietChild(child)
			if err := child.Start(); err != nil {
				return err
			}
			return child.Process.Release()
		case "remove-unreadable":
			return os.WriteFile(path, []byte("{transient unreadable state"), 0o600)
		}
	}
	if args[1] == "add-json" && (mode == "restore-committed-error" || mode == "add-committed-error") {
		return fmt.Errorf("add committed then failed")
	}
	if mode == "concurrent-add" && len(args) > 1 && args[1] == "add-json" {
		return fmt.Errorf("fake concurrent registration")
	}
	return nil
}
