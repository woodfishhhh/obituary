package mcp

import (
	"fmt"

	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
)

// Every response is an object, including partial failures. Receipts survive
// audit failures; returning a Go error to AddTool would discard those receipts.
type mcpReply struct {
	Data      any    `json:"data,omitempty"`
	Error     string `json:"error,omitempty"`
	Warning   string `json:"warning,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

func mcpResult(data any, err error) mcpReply {
	r := mcpReply{Data: data}
	if err != nil {
		r.Error = err.Error()
	}
	return r
}

func mcpCommittedResult(data any, err error, committed bool) mcpReply {
	r := mcpResult(data, err)
	if committed && cmdutil.OnlyAuditErrors(err) {
		r.Error, r.Warning = "", err.Error()
	}
	return r
}

func mcpItems(results []core.Result, err error) mcpReply {
	committed := len(results) > 0
	for _, r := range results {
		committed = committed && r.Action != "" && r.Error == ""
	}
	out := mcpCommittedResult(map[string]any{"results": results}, err, committed)
	if core.Failed(results) && out.Error == "" {
		out.Error = "one or more items failed; results contain individual errors"
	}
	return out
}

func mcpGraves(results []core.GraveResult, err error) mcpReply {
	committed := len(results) > 0
	for _, r := range results {
		committed = committed && (r.Action == core.ActionBuried || r.Action == core.ActionResurrected || r.Action == core.ActionRecycled) && r.Error == ""
	}
	for i := range results {
		if results[i].Action == core.ActionRecycled && results[i].Error == "" {
			results[i].Detail = cmdutil.RecycleRecovery
		}
	}
	out := mcpCommittedResult(map[string]any{"results": results}, err, committed)
	if core.GraveFailed(results) && out.Error == "" {
		out.Error = "one or more burials failed; results contain individual errors"
	}
	return out
}

func mcpDaemonWarning(reply mcpReply, home string) mcpReply {
	if err := cmdutil.EnsureDaemon(home, true); err != nil {
		warning := fmt.Sprintf("后台进程未能启动，寿命不会被自动处理；可以执行 obit daemon enable: %v", err)
		if reply.Warning != "" {
			reply.Warning += "; "
		}
		reply.Warning += warning
	}
	return reply
}
