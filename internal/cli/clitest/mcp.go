package clitest

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func ConnectMCP(t *testing.T, s *mcp.Server) *mcp.ClientSession {
	t.Helper()
	a, b := mcp.NewInMemoryTransports()
	server, err := s.Connect(t.Context(), a, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "obit-test", Version: "1"}, nil)
	session, err := client.Connect(t.Context(), b, &mcp.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func CallMCP(t *testing.T, session *mcp.ClientSession, name string, args any, failed bool, data any) *mcp.CallToolResult {
	t.Helper()
	// Only a hang guard: heavy recovery tests take several seconds alone and
	// more under the parallel full suite, so this must not act as a deadline.
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	r, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	if r.IsError != failed {
		t.Fatalf("%s: isError=%v, want %v: %+v", name, r.IsError, failed, r)
	}
	if data != nil {
		b, err := json.Marshal(r.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		var envelope struct{ Data json.RawMessage }
		if err := json.Unmarshal(b, &envelope); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(envelope.Data, data); err != nil {
			t.Fatalf("data: %v: %s", err, b)
		}
	}
	return r
}
