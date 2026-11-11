package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
)

func TestMCPCancelWaitingForLedger(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, tool := range []string{"obit_mark", "obit_bury", "obit_resurrect", "obit_extend", "obit_spare", "obit_mktemp"} {
		t.Run(tool, func(t *testing.T) {
			starts := clitest.IsolateRunner(t)
			repo, store := clitest.CommandRepo(t)
			path := filepath.Join(repo, "cancel.log")
			clitest.WriteFixture(t, path, "must remain")
			svc, err := cmdutil.Open()
			if err != nil {
				t.Fatal(err)
			}
			results, err := svc.Add([]string{path}, core.AddOptions{ExpiresAt: time.Now().Add(time.Hour)})
			if err != nil || core.Failed(results) {
				t.Fatalf("setup: %v %v", results, err)
			}
			ref := fmt.Sprint(results[0].Item.ID)
			args := map[string]any{"refs": []string{ref}}
			switch tool {
			case "obit_mark":
				args = map[string]any{"paths": []string{path}, "lifespan": "2d"}
			case "obit_bury":
				args = map[string]any{"paths": []string{path}}
			case "obit_resurrect":
				if _, err := svc.Bury([]string{path}, false, ""); err != nil {
					t.Fatal(err)
				}
			case "obit_extend":
				args["lifespan"] = "1d"
			case "obit_mktemp":
				args = map[string]any{"cwd": repo, "lifespan": "1d"}
			}
			before, err := store.Inspect()
			if err != nil {
				t.Fatal(err)
			}
			release, err := core.LockFile(filepath.Join(store.Home, "store.lock"), true)
			if err != nil {
				t.Fatal(err)
			}
			unlock := sync.OnceFunc(release)
			defer unlock()
			s := NewServer(svc, "0.1.0-dev")
			started := make(chan context.Context, 1)
			done := make(chan *mcp.CallToolResult, 1)
			s.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
				return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
					if method != "tools/call" {
						return next(ctx, method, req)
					}
					started <- ctx
					r, err := next(ctx, method, req)
					result, _ := r.(*mcp.CallToolResult)
					done <- result
					return r, err
				}
			})
			session := clitest.ConnectMCP(t, s)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			clientDone := make(chan error, 1)
			go func() {
				_, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
				clientDone <- err
			}()
			var requestCtx context.Context
			select {
			case requestCtx = <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("request did not start")
			}
			cancel()
			select {
			case <-requestCtx.Done():
			case <-time.After(3 * time.Second):
				t.Fatal("SDK did not deliver cancellation")
			}
			var result *mcp.CallToolResult
			select {
			case result = <-done:
			case <-time.After(500 * time.Millisecond):
				t.Error("canceled waiter did not finish while ledger remained locked")
				unlock()
				select {
				case result = <-done:
				case <-time.After(3 * time.Second):
					t.Fatal("handler stuck after unlock")
				}
			}
			unlock()
			if result == nil || !result.IsError {
				t.Error("canceled operation did not report error")
			}
			if err := <-clientDone; err == nil {
				t.Error("client cancellation succeeded unexpectedly")
			}
			after, err := store.Inspect()
			if err != nil || !reflect.DeepEqual(before, after) || len(*starts) != 0 {
				t.Fatalf("canceled request mutated ledger or runner: %v; starts=%v", err, *starts)
			}
			if tool != "obit_resurrect" {
				if got, err := os.ReadFile(path); err != nil || string(got) != "must remain" {
					t.Fatalf("canceled operation moved file: %q %v", got, err)
				}
			}
		})
	}
}

func TestMCPCancelBeforeCommit(t *testing.T) {
	for _, tool := range []string{"obit_mark", "obit_bury", "obit_resurrect"} {
		t.Run(tool, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			path := filepath.Join(repo, "before.log")
			clitest.WriteFixture(t, path, "preserved")
			svc, err := cmdutil.Open()
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Update(func(*core.State) error { return nil }); err != nil {
				t.Fatal(err)
			}
			args := map[string]any{"paths": []string{path}}
			if tool == "obit_mark" {
				args["lifespan"] = "1d"
			}
			if tool == "obit_resurrect" {
				graves, err := svc.Bury([]string{path}, false, "")
				if err != nil {
					t.Fatal(err)
				}
				args = map[string]any{"refs": []string{fmt.Sprint(graves[0].Burial.ID)}}
			}
			before := clitest.TreeSnapshot(t, filepath.Dir(repo))
			var cancel context.CancelFunc
			svc.FS = approvalFaultFS{FS: svc.FS, beforeIdentify: func(string) { cancel() }}
			server := NewServer(svc, "0.1.0-dev")
			server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
				return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
					if method == "tools/call" {
						ctx, cancel = context.WithCancel(ctx)
						defer cancel()
					}
					return next(ctx, method, req)
				}
			})
			clitest.CallMCP(t, clitest.ConnectMCP(t, server), tool, args, true, nil)
			// Lock files are synchronization metadata; no user data/ledger changes.
			after := clitest.TreeSnapshot(t, filepath.Dir(repo))
			for key := range before {
				if filepath.Ext(key) == ".lock" {
					delete(before, key)
				}
			}
			for key := range after {
				if filepath.Ext(key) == ".lock" {
					delete(after, key)
				}
			}
			// Directory mtimes may reflect the newly created lock files.
			for key := range before {
				if info, err := os.Stat(filepath.Join(filepath.Dir(repo), key)); err == nil && info.IsDir() {
					delete(before, key)
					delete(after, key)
				}
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatal("pre-commit cancellation changed files or ledger")
			}
		})
	}
}

func TestMCPCancelDuringMoveFinishesTransaction(t *testing.T) {
	for _, tool := range []string{"obit_bury", "obit_resurrect"} {
		t.Run(tool, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			path := filepath.Join(repo, "during.log")
			clitest.WriteFixture(t, path, "complete transaction")
			svc, err := cmdutil.Open()
			if err != nil {
				t.Fatal(err)
			}
			args := map[string]any{"paths": []string{path}}
			if tool == "obit_resurrect" {
				graves, err := svc.Bury([]string{path}, false, "")
				if err != nil {
					t.Fatal(err)
				}
				args = map[string]any{"refs": []string{fmt.Sprint(graves[0].Burial.ID)}}
			}
			var cancel context.CancelFunc
			svc.FS = concurrentBurialFS{FS: svc.FS, before: func(string) { cancel() }}
			server := NewServer(svc, "0.1.0-dev")
			server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
				return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
					if method == "tools/call" {
						ctx, cancel = context.WithCancel(ctx)
						defer cancel()
					}
					return next(ctx, method, req)
				}
			})
			r := clitest.CallMCP(t, clitest.ConnectMCP(t, server), tool, args, false, nil)
			data, _ := json.Marshal(r.StructuredContent)
			var reply mcpReply
			if err := json.Unmarshal(data, &reply); err != nil || reply.Warning != "" {
				t.Fatalf("cancellation interrupted committed audit: %s %v", data, err)
			}
			st, err := store.Inspect()
			if err != nil {
				t.Fatal(err)
			}
			wantBurials := 1
			if tool == "obit_resurrect" {
				wantBurials = 0
			}
			if len(st.Items) != 0 || len(st.Burials) != wantBurials {
				t.Fatalf("cancellation left incomplete ledger: %+v", st)
			}
			if pending, err := svc.PendingGraves(); err != nil || len(pending) != 0 {
				t.Fatalf("cancellation left recovery manifest: %v %v", pending, err)
			}
			restored := path
			if wantBurials != 0 {
				restored = st.Burials[0].Grave
			}
			if got, err := os.ReadFile(restored); err != nil || string(got) != "complete transaction" {
				t.Fatalf("transaction lost content: %q %v", got, err)
			}
		})
	}
}

func TestMCPLargeIDRoundTrip(t *testing.T) {
	clitest.IsolateRunner(t)
	repo, store := clitest.CommandDir(t)
	const id int64 = 9007199254740993
	if err := store.Update(func(st *core.State) error { st.NextID = id; return nil }); err != nil {
		t.Fatal(err)
	}
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	// Capture the protocol before the client's generic any/float64 decoding.
	// CallTool's text fallback must contain exactly the same integer as the wire.
	var wire bytes.Buffer
	a, b := mcp.NewInMemoryTransports()
	server, err := NewServer(svc, "0.1.0-dev").Connect(t.Context(), a, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client, err := mcp.NewClient(&mcp.Implementation{Name: "precision", Version: "1"}, nil).Connect(t.Context(), &mcp.LoggingTransport{Transport: b, Writer: &wire}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	path := filepath.Join(repo, "large.log")
	clitest.WriteFixture(t, path, "keep")
	r, err := client.CallTool(t.Context(), &mcp.CallToolParams{Name: "obit_mark", Arguments: map[string]any{"paths": []string{path}, "lifespan": "1h"}})
	if err != nil || r.IsError {
		t.Fatalf("large ID call: %+v %v", r, err)
	}
	var envelope struct {
		Data struct{ Results []core.Result }
	}
	if err := json.Unmarshal([]byte(r.Content[0].(*mcp.TextContent).Text), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.Results) != 1 || envelope.Data.Results[0].Item.ID != id {
		t.Fatalf("MCP rounded large ID in CallTool result: %+v", envelope.Data.Results)
	}
	var found bool
	for _, line := range strings.Split(wire.String(), "\n") {
		if !strings.HasPrefix(line, "read: ") {
			continue
		}
		var response struct {
			Result struct{ StructuredContent json.RawMessage }
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "read: ")), &response); err != nil {
			t.Fatal(err)
		}
		if len(response.Result.StructuredContent) == 0 {
			continue
		}
		if err := json.Unmarshal(response.Result.StructuredContent, &envelope); err != nil {
			t.Fatal(err)
		}
		found = len(envelope.Data.Results) == 1 && envelope.Data.Results[0].Item.ID == id
	}
	if !found {
		t.Fatal("MCP rounded large ID in structured wire result")
	}
	for _, after := range []int64{id - 1, id} {
		r, err := client.CallTool(t.Context(), &mcp.CallToolParams{Name: "obit_list", Arguments: map[string]any{"after": fmt.Sprint(after)}})
		if err != nil || r.IsError {
			t.Fatalf("large cursor call: %+v %v", r, err)
		}
		var page struct{ Data struct{ Items []core.Item } }
		if err := json.Unmarshal([]byte(r.Content[0].(*mcp.TextContent).Text), &page); err != nil {
			t.Fatal(err)
		}
		if after < id && (len(page.Data.Items) != 1 || page.Data.Items[0].ID != id) || after == id && len(page.Data.Items) != 0 {
			t.Fatal("large ID cursor rounded across page boundary")
		}
	}
	r, err = client.CallTool(t.Context(), &mcp.CallToolParams{Name: "obit_spare", Arguments: map[string]any{"refs": []string{fmt.Sprint(id)}}})
	if err != nil || r.IsError {
		t.Fatalf("large ID could not be used for a subsequent call: %+v %v", r, err)
	}
}
