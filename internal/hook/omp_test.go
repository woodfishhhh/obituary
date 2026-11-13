package hook

import "github.com/woodfishhhh/obituary/internal/testenv"

import "testing"

func TestOMPBridge(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	testPluginBridge(t, "omp")
}

func TestOMPAndPiSessionNamespaces(t *testing.T) {
	t.Parallel()
	var sessions []string
	for _, name := range []string{"pi", "omp"} {
		agent, _ := Lookup(name)
		input := BridgeRequest{Version: 1, Agent: name, Event: "Stop", HostEvent: "agent_end", Session: "same-id", Cwd: testenv.NativeFixture(`C:\repo`), Local: true}
		ev, err := Decode(agent.Adapter, pluginJSON(t, input), Invocation{Agent: name})
		if err != nil || ev.Session != name+":same-id" {
			t.Fatalf("%s session: %+v %v", name, ev, err)
		}
		sessions = append(sessions, ev.Session)
	}
	if sessions[0] == sessions[1] {
		t.Fatal("Pi and OMP share a session namespace")
	}
}
