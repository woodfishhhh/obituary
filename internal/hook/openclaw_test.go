package hook

import (
	"bytes"
	"testing"
)

func TestOpenClawBridge(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	testPluginBridge(t, "openclaw")
}

func TestOpenClawObservationDoesNotAcknowledgeNotices(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, event := range []string{"PostToolUse", "SessionStart"} {
		t.Run(event, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			path := e.write("debug.log")
			if _, _, err := e.engine.Observe([]string{path}, "seed"); err != nil {
				t.Fatal(err)
			}
			success := true
			input := BridgeRequest{Version: 1, Agent: "openclaw", Event: event, Session: "s", Cwd: e.root,
				Local: true, Success: &success, Paths: []string{path}, ContextDelivery: true}
			agent, _ := Lookup("openclaw")
			ev, err := Decode(agent.Adapter, pluginJSON(t, input), Invocation{Agent: "openclaw"})
			if err != nil || !ev.DeferContext {
				t.Fatalf("observation callback advertised context delivery: %+v %v", ev, err)
			}
			var out bytes.Buffer
			RunEvent(agent, e.engine, ev, &out, &Runner{})
			if response := pluginResponse(t, out.Bytes()); response != (BridgeResponse{Version: 1}) {
				t.Fatalf("observation callback emitted context: %+v", response)
			}
			state := e.state()
			if len(state.Proposals) != 1 || len(state.Proposals[0].Notified) != 0 {
				t.Fatalf("undelivered proposal acknowledged: %+v", state.Proposals)
			}
		})
	}
}
