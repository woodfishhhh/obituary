package hook

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
)

func TestAdapterRegistry(t *testing.T) {
	for _, name := range []string{"claude", "codex"} {
		agent, ok := Lookup(name)
		if !ok || agent.Name != name || agent.Adapter == nil {
			t.Fatalf("missing adapter %q: %+v", name, agent)
		}
	}
	if _, ok := Lookup("unknown"); ok {
		t.Fatal("unknown agent found")
	}
	all := Agents()
	for i := 1; i < len(all); i++ {
		if all[i-1].Name >= all[i].Name {
			t.Fatal("unordered registry")
		}
	}
	all[0].Name = "changed"
	if Agents()[0].Name == "changed" {
		t.Fatal("caller changed registry")
	}
	for _, agent := range []Agent{{}, {Name: "Bad", Adapter: claudeAdapter{}}, {Name: "audit", Adapter: claudeAdapter{}}, {Name: "claude", Adapter: claudeAdapter{}}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("invalid registration accepted: %+v", agent)
				}
			}()
			Register(agent)
		}()
	}
}

func TestAdapterMatchesLegacyEntryPoints(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"claude", "codex"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			agent, _ := Lookup(name)
			input := []byte(`{"hook_event_name":"PreToolUse","cwd":"C:\\repo","tool_name":"Bash","tool_input":{"command":"rm scratch.log","shell":"bash"}}`)
			ev, err := agent.Adapter.Decode(input, Invocation{Agent: name})
			if err != nil || ev.Name != "PreToolUse" || ev.Command != "rm scratch.log" {
				t.Fatalf("decode: %+v %v", ev, err)
			}
			e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: t.TempDir()}}}
			var direct, decoded bytes.Buffer
			if name == "claude" {
				Claude(e, bytes.NewReader(input), &direct, nil)
			} else {
				Codex(e, bytes.NewReader(input), &direct, nil)
			}
			RunEvent(agent, e, ev, &decoded, nil)
			if !bytes.Equal(direct.Bytes(), decoded.Bytes()) || !bytes.Contains(decoded.Bytes(), []byte(`"permissionDecision":"deny"`)) {
				t.Fatalf("legacy mismatch: %s / %s", direct.Bytes(), decoded.Bytes())
			}
			if got := agent.Adapter.Fallback(Event{}); len(got) != 0 {
				t.Fatalf("legacy fallback changed: %s", got)
			}
		})
	}
}

func TestAdapterReadLimitAndBOM(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"claude", "codex"} {
		agent, _ := Lookup(name)
		plain := []byte(`{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"echo ok"}}`)
		a, err := Decode(agent.Adapter, plain, Invocation{Agent: name})
		if err != nil {
			t.Fatal(err)
		}
		b, err := Decode(agent.Adapter, append([]byte("\xef\xbb\xbf"), plain...), Invocation{Agent: name})
		if err != nil || !reflect.DeepEqual(a, b) {
			t.Fatalf("BOM changed event: %+v %+v %v", a, b, err)
		}
	}
	for _, length := range []int{MaxInput, MaxInput + 1} {
		data, err := ReadInput(strings.NewReader(strings.Repeat(" ", length)))
		if length == MaxInput && (err != nil || len(data) != length) || length > MaxInput && err == nil {
			t.Fatalf("length=%d read=%d err=%v", length, len(data), err)
		}
	}
}

type brokenAdapter struct{ stage string }

func (a brokenAdapter) Decode([]byte, Invocation) (Event, error) {
	if a.stage == "panic" {
		panic("decoder panic")
	}
	return Event{}, errors.New("decode failed")
}
func (a brokenAdapter) Encode(Event, Result) ([]byte, error) {
	return nil, errors.New("encode failed")
}
func (a brokenAdapter) Fallback(Event) []byte { return []byte("{}\n") }

type failedReader struct{}

func (failedReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestAdapterFailureUsesNeutralOutput(t *testing.T) {
	for _, stage := range []string{"panic", "decode", "read", "disabled"} {
		t.Run(stage, func(t *testing.T) {
			name := "test-adapter-" + stage
			Register(Agent{Name: name, Adapter: brokenAdapter{stage: stage}})
			t.Cleanup(func() { adapters.Lock(); delete(adapters.byName, name); adapters.Unlock() })
			e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: t.TempDir()}}}
			var out bytes.Buffer
			var in io.Reader = strings.NewReader("{}")
			if stage == "read" {
				in = failedReader{}
			}
			if stage == "disabled" {
				t.Setenv("OBIT_DISABLED", "1")
			}
			Run(name, e, in, &out, nil)
			if out.String() != "{}\n" {
				t.Fatalf("missing neutral output: %q", out.String())
			}
		})
	}
	var out bytes.Buffer
	e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: t.TempDir()}}}
	RunEvent(Agent{Name: "broken", Adapter: brokenAdapter{}}, e, Event{Name: "PreToolUse", SkipReason: "unsupported"}, &out, nil)
	if out.String() != "{}\n" {
		t.Fatalf("encode failure lost neutral response: %q", out.String())
	}
}
