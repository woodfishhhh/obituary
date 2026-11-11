package mcp

import (
	"errors"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
	"github.com/woodfishhhh/obituary/internal/project"
)

type mcpMarkInput struct {
	Paths    []string `json:"paths" jsonschema:"Absolute Windows paths to register"`
	Lifespan string   `json:"lifespan,omitempty" jsonschema:"Positive lifespan such as 2h or 3d; exactly one of lifespan and at is supplied"`
	At       string   `json:"at,omitempty" jsonschema:"Expiry accepted by obit add --at; exactly one of lifespan and at is supplied"`
	Note     string   `json:"note,omitempty"`
	Force    bool     `json:"force,omitempty" jsonschema:"Allows directories containing a Git repository; other Guard checks still apply"`
}

type mcpTempInput struct {
	Cwd      string `json:"cwd" jsonschema:"Absolute Windows working directory used to find the Git project"`
	Lifespan string `json:"lifespan" jsonschema:"Positive lifespan such as 2h or 3d"`
	Name     string `json:"name,omitempty"`
	Session  string `json:"session,omitempty"`
}

type mcpBuryInput struct {
	Paths []string `json:"paths" jsonschema:"Absolute Windows paths to bury"`
	Force bool     `json:"force,omitempty" jsonschema:"Allows directories containing a Git repository; other Guard checks still apply"`
	Note  string   `json:"note,omitempty"`
}

type mcpRefsInput struct {
	Refs []string `json:"refs" jsonschema:"Positive ledger IDs (optionally prefixed with #) or absolute Windows paths"`
}

type mcpGraveyardInput struct {
	mcpPageInput
	Here string `json:"here,omitempty" jsonschema:"Optional absolute Windows directory; include only original paths in this directory and its descendants"`
}

type mcpExtendInput struct {
	Refs     []string `json:"refs" jsonschema:"Positive ledger IDs or absolute Windows paths"`
	Lifespan string   `json:"lifespan" jsonschema:"Positive additional lifespan"`
}

// MCP has no caller cwd or reliable session. Only a common Git project of all
// explicit operands can scope its batch; never borrow the server's directory.
func mcpBurialService(svc *core.Service, paths []string) (*core.Service, error) {
	root, common := "", true
	for i, path := range paths {
		p, found, err := project.Find(path)
		if err != nil {
			return nil, err
		}
		if !found || i > 0 && p.Root != root {
			common = false
		}
		if i == 0 {
			root = p.Root
		}
	}
	if !common {
		root = ""
	}
	return svc.BeginBurial("", root)
}

func NewServer(svc *core.Service, version string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "obituary", Version: version}, nil)
	addMCPTool(s, svc, "obit_mark", "Registers absolute paths with a lifespan, as obit add does. Re-registering resets expiry. Guard and identity checks apply.", false, markMCP)
	addMCPTool(s, svc, "obit_mktemp", "Creates and registers a temporary directory with a lifespan. The explicit absolute cwd selects the Git project; outside Git the system temporary directory is used.", false, mktempMCP)
	buryDescription := "Moves absolute paths into the restorable graveyard immediately, with the same Guard and identity checks as obit bury."
	if svc.Mode == core.ModeRecycle {
		buryDescription = "Moves absolute paths to the system Recycle Bin immediately, with the same Guard and identity checks as obit bury. Restore from the Recycle Bin; obit_resurrect cannot restore these files."
	}
	addMCPTool(s, svc, "obit_bury", buryDescription, false, buryMCP)
	addMCPTool(s, svc, "obit_resurrect", "Restores verified burials by ID or absolute original path. Existing files are never overwritten; the result reports the restored path.", false, resurrectMCP)
	addMCPTool(s, svc, "obit_extend", "Extends tracked lifespans by ID or absolute path, counted from now when already due.", false, extendMCP)
	addMCPTool(s, svc, "obit_spare", "Stops tracking IDs or absolute paths and leaves files in place.", false, spareMCP)
	addMCPTool(s, svc, "obit_list", "Lists tracked items in ascending ID order; pass nextAfter as after and version to continue. changed warns when the full set changes; restart for a complete view. total counts this read.", true, listMCP)
	addMCPTool(s, svc, "obit_graveyard", "Lists burials and pending manifests together in ascending ID order; reconciles verified interrupted restores and reports their actual paths. Optional here filters original paths by an absolute directory, including descendants. Pass nextAfter as after and version to continue with the same here. changed warns when the full set changes; restart for a complete view. total counts this read. Permanent purging is a user decision.", false, graveyardMCP)
	addMCPTool(s, svc, "obit_proposals", "Lists proposals in ascending ID order; pass nextAfter as after and version to continue. changed warns when the full set changes; restart for a complete view. total counts this read. Approval and rejection belong to the user, who executes obit approve or obit reject; this server cannot decide them.", true, proposalsMCP)
	addMCPTool(s, svc, "obit_stats", "Reports tracked items, graveyard size, lifetime counters and delete guard counts.", true, statsMCP)
	return s
}

func markMCP(svc *core.Service, in mcpMarkInput) mcpReply {
	if err := mcpPaths(in.Paths, false); err != nil {
		return mcpResult(nil, err)
	}
	if (in.Lifespan == "") == (in.At == "") {
		return mcpResult(nil, errors.New("exactly one of lifespan and at is required"))
	}
	now := time.Now()
	var expires time.Time
	if in.At != "" {
		var err error
		expires, err = core.ParseAt(in.At, now)
		if err != nil {
			return mcpResult(nil, err)
		}
	} else {
		d, err := core.ParseDuration(in.Lifespan)
		if err != nil {
			return mcpResult(nil, err)
		}
		expires = now.Add(d)
	}
	results, err := svc.Add(in.Paths, core.AddOptions{ExpiresAt: expires, Force: in.Force, Note: in.Note})
	reply := mcpItems(results, err)
	if cmdutil.AnyOK(results) {
		reply = mcpDaemonWarning(reply, svc.Store.Home)
	}
	return reply
}

func mktempMCP(svc *core.Service, in mcpTempInput) mcpReply {
	if err := mcpPaths([]string{in.Cwd}, false); err != nil {
		return mcpResult(nil, err)
	}
	d, err := core.ParseDuration(in.Lifespan)
	if err != nil {
		return mcpResult(nil, err)
	}
	path, item, err := app.Engine(svc).MkTemp(d, in.Name, in.Cwd, in.Session)
	reply := mcpCommittedResult(map[string]any{"path": path, "item": item}, err, item != nil)
	if item != nil {
		reply = mcpDaemonWarning(reply, svc.Store.Home)
	}
	return reply
}

func buryMCP(svc *core.Service, in mcpBuryInput) mcpReply {
	if err := mcpPaths(in.Paths, false); err != nil {
		return mcpResult(nil, err)
	}
	svc, err := mcpBurialService(svc, in.Paths)
	if err != nil {
		return mcpResult(nil, err)
	}
	results, err := svc.Bury(in.Paths, in.Force, in.Note)
	reply := mcpGraves(results, err)
	for _, r := range results {
		if r.Action == core.ActionBuried || r.Action == core.ActionRecoveryPending {
			reply = mcpDaemonWarning(reply, svc.Store.Home)
			break
		}
	}
	return reply
}

func resurrectMCP(svc *core.Service, in mcpRefsInput) mcpReply {
	if err := mcpReferences(in.Refs, true, true); err != nil {
		return mcpResult(nil, err)
	}
	results, err := svc.Resurrect(in.Refs)
	return mcpGraves(results, err)
}

func extendMCP(svc *core.Service, in mcpExtendInput) mcpReply {
	if err := mcpPaths(in.Refs, true); err != nil {
		return mcpResult(nil, err)
	}
	d, err := core.ParseDuration(in.Lifespan)
	if err != nil {
		return mcpResult(nil, err)
	}
	results, err := svc.Extend(in.Refs, d)
	return mcpItems(results, err)
}

func spareMCP(svc *core.Service, in mcpRefsInput) mcpReply {
	if err := mcpPaths(in.Refs, true); err != nil {
		return mcpResult(nil, err)
	}
	results, err := svc.Cancel(in.Refs)
	return mcpItems(results, err)
}

func listMCP(svc *core.Service, in mcpPageInput) mcpReply {
	st, err := svc.Store.Read()
	return mcpPageResult("items", st.Items, func(it core.Item) int64 { return it.ID }, in, err)
}

func graveyardMCP(svc *core.Service, in mcpGraveyardInput) mcpReply {
	if in.Here != "" {
		if err := mcpPaths([]string{in.Here}, false); err != nil {
			return mcpResult(nil, err)
		}
	}
	view, err := svc.ReadGraveyard()
	if view == nil {
		return mcpResult(nil, err)
	}
	if in.Here != "" {
		if filterErr := svc.FilterGraveyard(view, in.Here); filterErr != nil {
			return mcpResult(nil, filterErr)
		}
	}
	reply := mcpGravePage(view.Burials, view.Pending, in.mcpPageInput, err)
	if data, ok := reply.Data.(map[string]any); ok && len(view.Restored) > 0 {
		data["restored"], data["restoredCount"] = view.Restored[:min(200, len(view.Restored))], len(view.Restored)
		if len(view.Restored) > 200 {
			reply.Warning += " Recovered restore receipts limited to 200; remaining actual paths are in obit log."
		}
	}
	return reply
}

func proposalsMCP(svc *core.Service, in mcpPageInput) mcpReply {
	st, err := svc.Store.Read()
	return mcpPageResult("proposals", core.ExistingProposals(st.Proposals), func(p core.Proposal) int64 { return p.ID }, in, err)
}

func statsMCP(svc *core.Service, _ struct{}) mcpReply {
	sum, err := svc.Summarize()
	if err != nil {
		return mcpResult(nil, err)
	}
	counts, err := guardrm.ReadCounts(svc.Store.Home)
	counts.Buried += sum.Lifetime.DeleteBuried
	return mcpResult(struct {
		core.Summary
		DeleteGuard guardrm.Counts `json:"deleteGuard"`
	}{sum, counts}, err)
}
