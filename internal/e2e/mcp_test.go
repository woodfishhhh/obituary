//go:build windows

package e2e_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestMCPStdioProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	repo, _ := commandRepo(t) // builds both binaries, runs only an isolated resident
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath(t, "obit"), "mcp")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stdin.Close()
		cancel()
		cmd.Wait()
	})
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	encoder := json.NewEncoder(stdin)
	nextID := 0
	rpc := func(method string, params any) json.RawMessage {
		t.Helper()
		nextID++
		if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": nextID, "method": method, "params": params}); err != nil {
			t.Fatal(err)
		}
		if !scanner.Scan() {
			t.Fatalf("missing %s response: %v", method, scanner.Err())
		}
		var response struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      int             `json:"id"`
			Result  json.RawMessage `json:"result"`
			Error   json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil || response.JSONRPC != "2.0" || response.ID != nextID || response.Error != nil {
			t.Fatalf("stdout was not the expected MCP protocol response: %s (%v)", scanner.Bytes(), err)
		}
		return response.Result
	}
	rpc("initialize", map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "e2e", "version": "1"}})
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}); err != nil {
		t.Fatal(err)
	}
	var list struct{ Tools []struct{ Name string } }
	if err := json.Unmarshal(rpc("tools/list", map[string]any{}), &list); err != nil || len(list.Tools) != 10 {
		t.Fatalf("tools/list: %+v %v", list, err)
	}
	call := func(name string, arguments any, failed bool, target any) {
		t.Helper()
		var result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Data json.RawMessage `json:"data"`
			} `json:"structuredContent"`
		}
		data := rpc("tools/call", map[string]any{"name": name, "arguments": arguments})
		if err := json.Unmarshal(data, &result); err != nil || result.IsError != failed {
			t.Fatalf("%s: %s %v", name, data, err)
		}
		if target != nil {
			if err := json.Unmarshal(result.StructuredContent.Data, target); err != nil {
				t.Fatal(err)
			}
		}
	}
	path := filepath.Join(repo, "stdio.log")
	writeFixture(t, path, "stdio round trip")
	var added struct{ Results []core.Result }
	call("obit_mark", map[string]any{"paths": []string{path}, "lifespan": "1d"}, false, &added)
	if len(added.Results) != 1 || added.Results[0].Item == nil {
		t.Fatalf("mark: %+v", added)
	}
	id := fmt.Sprint(added.Results[0].Item.ID)
	call("obit_bury", map[string]any{"paths": []string{path}}, false, nil)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("stdio bury left file in place", err)
	}
	call("obit_resurrect", map[string]any{"refs": []string{id}}, false, nil)
	if data, err := os.ReadFile(path); err != nil || string(data) != "stdio round trip" {
		t.Fatalf("stdio resurrection: %q %v", data, err)
	}
	call("obit_bury", map[string]any{"paths": []string{"stdio.log"}}, true, nil)
	call("obit_bury", map[string]any{"paths": []string{repo}}, true, nil)
	call("obit_list", map[string]any{}, false, nil)
	stdin.Close()
	for scanner.Scan() {
		if !json.Valid(scanner.Bytes()) {
			t.Fatalf("non-protocol stdout: %q", scanner.Bytes())
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil || stderr.Len() != 0 {
		t.Fatalf("MCP exit: %v; stderr: %s", err, stderr.String())
	}
}
