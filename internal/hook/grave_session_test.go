package hook

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestGraveNoticeAcknowledgedAfterOutput(t *testing.T) {
	e, root, b := noticeFixture(t, false)
	agent, _ := Lookup("claude")
	ev := Event{Name: "PreToolUse", Session: "s", Cwd: root, References: []string{b.Path}, NoticeOnly: true}
	RunEvent(agent, e, ev, deliveryWriter(func(p []byte) (int, error) { return 0, io.ErrClosedPipe }), nil)
	if len(readGraveSession(e.Svc.Store.Home, Event{Host: "claude", Session: "s"}).Seen) != 0 {
		t.Fatal("failed output acknowledged")
	}
	var out bytes.Buffer
	RunEvent(agent, e, ev, &out, nil)
	if !strings.Contains(out.String(), "obit resurrect 42") {
		t.Fatal(out.String())
	}
}

func TestGraveNoticeDeferredAdapters(t *testing.T) {
	for _, name := range []string{"pi", "omp", "openclaw", "opencode", "cline", "kilocode", "hermes", "copilot", "deepseek-harness", "gemini", "cursor", "qoder", "qoder-cn", "codebuddy", "codebuddy-ide", "minimax-code", "vibe", "zcode", "antigravity"} {
		t.Run(name, func(t *testing.T) {
			e, root, b := noticeFixture(t, false)
			agent, ok := Lookup(name)
			if !ok {
				t.Fatal(name)
			}
			ev := Event{Host: name, Name: "PreToolUse", Session: "session", Cwd: root, References: []string{b.Path}, DeferContext: true, NoticeOnly: true}
			var out bytes.Buffer
			RunEvent(agent, e, ev, &out, nil)
			if strings.Contains(out.String(), "obit resurrect") {
				t.Fatal("pre emitted deferred context", out.String())
			}
			pending := readGraveSession(e.Svc.Store.Home, ev)
			if len(pending.Pending) != 1 || len(pending.Seen) != 0 {
				t.Fatalf("not queued: %+v", pending)
			}
			// Exercise the common delivery path with the host's actual encoder, avoiding
			// unrelated Git/observation work; the encoder remains the context authority.
			ev.Name, ev.HostEvent, ev.DeferContext, ev.References = "PostToolUse", "PostToolUse", false, nil
			if name == "openclaw" || name == "kilocode" || name == "deepseek-harness" || name == "qoder" || name == "qoder-cn" || name == "antigravity" {
				ev.Name = "Context"
			}
			d := delivery{}
			prepareGraveNotices(e.Svc.Store, ev, &d)
			defer d.close()
			encoded, err := agent.Adapter.Encode(ev, Result{Context: d.text()})
			if err != nil || !strings.Contains(string(encoded), "obit resurrect 42") {
				t.Fatalf("%s: %s %v", name, encoded, err)
			}
			d.acknowledge(ev.Session)
			if len(readGraveSession(e.Svc.Store.Home, ev).Seen) != 1 {
				t.Fatal("delivered notice not acknowledged")
			}
		})
	}
}
