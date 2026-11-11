package hook

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"

	"github.com/woodfishhhh/obituary/internal/auto"
)

const MaxInput = 4 << 20

// ErrSkip means the adapter recognized an event that this build cannot process.
// It preserves the host's neutral response without creating an error log.
var ErrSkip = errors.New("hook event skipped")

// ReadInput rejects oversized payloads, including valid JSON followed by padding.
func ReadInput(in io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(in, MaxInput+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxInput {
		return nil, fmt.Errorf("hook input exceeds %d bytes", MaxInput)
	}
	return data, nil
}

// Decode applies the common input bound and UTF-8 BOM handling before the host
// adapter so every host receives the same size and encoding safeguards.
func Decode(adapter Adapter, raw []byte, invocation Invocation) (Event, error) {
	if len(raw) > MaxInput {
		return Event{}, fmt.Errorf("hook input exceeds %d bytes", MaxInput)
	}
	return adapter.Decode(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf")), invocation)
}

// Invocation carries trusted command-line metadata, separate from the payload.
// Event is an optional host event name for hosts whose payload omits it.
type Invocation struct {
	Agent string
	Event string
}

// Result contains only a denial or additional context. It cannot grant permission
// or replace tool input; the host retains its own permission checks.
type Result struct {
	Deny    bool   `json:"deny,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Context string `json:"context,omitempty"`
}

// Adapter translates one host protocol. Decode does no I/O. Fallback must work
// with a zero Event and return that host's neutral response, including on errors.
// Encode returns complete JSON bytes, or nil for hosts that accept silence.
type Adapter interface {
	Decode(raw []byte, invocation Invocation) (Event, error)
	Encode(event Event, result Result) ([]byte, error)
	Fallback(event Event) []byte
}

// Agent is a hook protocol and its verified coverage. Register from each agent's
// init function so adding an adapter does not require editing a shared switch.
type Agent struct {
	Name         string
	Adapter      Adapter
	Capabilities Capabilities
}

var adapters = struct {
	sync.RWMutex
	byName map[string]Agent
}{byName: make(map[string]Agent)}

// Register rejects programming errors at startup. Runtime payloads never register
// adapters. Names use the same lowercase spelling as the CLI subcommand.
func Register(agent Agent) {
	if !validAgentName(agent.Name) || agent.Adapter == nil {
		panic("hook: invalid agent registration: " + agent.Name)
	}
	adapters.Lock()
	defer adapters.Unlock()
	if _, exists := adapters.byName[agent.Name]; exists {
		panic("hook: duplicate agent registration: " + agent.Name)
	}
	agent.Capabilities.GraveNotice = graveNoticeSupport(agent)
	adapters.byName[agent.Name] = agent
}

func validAgentName(name string) bool {
	if name == "" || name == "finish" || name == "audit" {
		return false
	}
	for i, c := range name {
		if c >= 'a' && c <= 'z' || i > 0 && (c >= '0' && c <= '9' || c == '-') {
			continue
		}
		return false
	}
	return true
}

func Lookup(name string) (Agent, bool) {
	adapters.RLock()
	defer adapters.RUnlock()
	agent, ok := adapters.byName[name]
	return agent, ok
}

// Agents returns a sorted snapshot; callers cannot mutate the registry through it.
func Agents() []Agent {
	adapters.RLock()
	defer adapters.RUnlock()
	agents := make([]Agent, 0, len(adapters.byName))
	for _, agent := range adapters.byName {
		agents = append(agents, agent)
	}
	slices.SortFunc(agents, func(a, b Agent) int { return bytes.Compare([]byte(a.Name), []byte(b.Name)) })
	return agents
}

// Run is the compatibility entry point for callers that already own an engine.
// The CLI decodes first so PreToolUse never opens the service.
func Run(name string, e *auto.Engine, in io.Reader, out io.Writer, runner *Runner) {
	agent, ok := Lookup(name)
	if !ok {
		Logf(e.Svc.Store.Home, "%s: unknown adapter", name)
		return
	}
	var ev Event
	defer func() {
		if r := recover(); r != nil {
			Logf(e.Svc.Store.Home, "%s: panic: %v", name, r)
			WriteFallback(agent.Adapter, ev, out)
		}
	}()
	if Disabled() {
		WriteFallback(agent.Adapter, ev, out)
		return
	}
	data, err := ReadInput(in)
	if err == nil {
		ev, err = Decode(agent.Adapter, data, Invocation{Agent: name})
	}
	if err != nil {
		if !errors.Is(err, ErrSkip) {
			LogInputError(e.Svc.Store.Home, Invocation{Agent: name}, data, err)
		}
		WriteFallback(agent.Adapter, ev, out)
		return
	}
	RunEvent(agent, e, ev, out, runner)
}

// RunEvent handles an already decoded event and emits exactly one host response.
func RunEvent(agent Agent, e *auto.Engine, ev Event, out io.Writer, runner *Runner) {
	ev.Host = agent.Name
	if len(ev.References) == 0 && ev.Name == "PostToolUse" {
		ev.References = slices.Clone(ev.Paths)
		ev.Writing = true
	}
	defer func() {
		if r := recover(); r != nil {
			Logf(e.Svc.Store.Home, "%s: panic: %v", agent.Name, r)
			WriteFallback(agent.Adapter, ev, out)
		}
	}()
	if err := configureAgentGuard(agent.Name, &ev); err != nil {
		Logf(e.Svc.Store.Home, "%s guard configuration: %v", agent.Name, err)
		WriteFallback(agent.Adapter, ev, out)
		return
	}
	// Claude/Codex keep their existing per-write notice selection; new hosts
	// can drain observation-only callbacks on any context-capable event.
	context, err := prepareEvent(e, ev, runner, agent.Name != "claude" && agent.Name != "codex")
	defer context.close()
	if err != nil {
		LogEventError(e.Svc.Store.Home, agent.Name, ev, err)
		WriteFallback(agent.Adapter, ev, out)
		return
	}
	text := context.text()
	result := Result{Context: text}
	if context.deny {
		result = Result{Deny: true, Reason: text}
	}
	data, err := agent.Adapter.Encode(ev, result)
	if err != nil {
		Logf(e.Svc.Store.Home, "%s: encode output: %v", agent.Name, err)
		WriteFallback(agent.Adapter, ev, out)
		return
	}
	if n, err := out.Write(data); err != nil || n != len(data) {
		if err == nil {
			err = io.ErrShortWrite
		}
		Logf(e.Svc.Store.Home, "%s: write output: %v", agent.Name, err)
		return
	}
	if len(data) > 0 {
		context.acknowledge(ev.Session)
	}
}

// WriteFallback is best effort, including when an adapter or the output panics.
func WriteFallback(adapter Adapter, ev Event, out io.Writer) {
	defer func() { _ = recover() }()
	_, _ = out.Write(adapter.Fallback(ev))
}

// EncodeJSON preserves literal shell characters in suggestions, like the existing
// Claude encoder. The standard library owns protocol escaping and framing.
func EncodeJSON(value any) ([]byte, error) {
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return nil, fmt.Errorf("hook JSON: %w", err)
	}
	return out.Bytes(), nil
}
