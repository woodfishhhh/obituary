package hook

// Lingma's five-event IDE protocol is distinct from the Qoder CN CLI.
// docs.qoder.cn/user-guide/hooks and the legacy Aliyun hook reference document
// the common envelope and tool aliases; no SessionStart/SessionEnd is assumed.
type lingmaAdapter struct{ qoderAdapter }

func (a lingmaAdapter) Decode(raw []byte, invocation Invocation) (Event, error) {
	ev, err := a.qoderAdapter.Decode(raw, invocation)
	if ev.Name == "Context" {
		// The five-event reference describes context injection as a use case,
		// but does not specify its output channel. Do not acknowledge notices
		// using a field documented only for the separate international IDE.
		ev.DeferContext = true
	}
	return ev, err
}

func (a lingmaAdapter) Encode(ev Event, result Result) ([]byte, error) {
	if ev.Name == "Context" {
		return nil, nil
	}
	return a.qoderAdapter.Encode(ev, result)
}

func init() {
	caps := qoderCapabilities()
	caps.SessionStart = Support{Detail: "No SessionStart in the documented five-event Lingma IDE protocol"}
	caps.SessionEnd = Support{Detail: "No SessionEnd in the documented five-event Lingma IDE protocol"}
	caps.Stop = Support{Level: Limited, Detail: "Silent turn finish; no session-start snapshot"}
	caps.WriteObservation.Detail = "Successful PostToolUse for Write/Edit/create_file/search_replace with file_path; compatibility fixture, not a recorded client payload"
	caps.Context = Support{Detail: "Post notices remain unacknowledged; UserPromptSubmit JSON output is unverified, inspect obit proposals manually"}
	caps.MCP = Support{Level: Limited, Detail: "IDE/plugin Settings > MCP Servers > custom JSON; physical target unverified"}
	Register(Agent{Name: "lingma", Adapter: lingmaAdapter{qoderAdapter{name: "lingma"}}, Capabilities: caps})
}
