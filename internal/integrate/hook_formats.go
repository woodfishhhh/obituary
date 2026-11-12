package integrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// HookShape describes the event container and its entries. RootHooks uses the
// same matcher groups as GroupedHooks, with events directly at the file root.
type HookShape string

const (
	GroupedHooks HookShape = "grouped"
	FlatHooks    HookShape = "flat"
	RootHooks    HookShape = "root"
)

type HookRegistration struct {
	Event, Matcher string
	Handler        json.RawMessage
	Optional       bool
}

// HookConfig describes native command handlers. CommandKeys defaults to
// ["command"]; every populated command field must be owned. Event arguments
// are accepted only from EventArgs, usually the host's complete event list.
type HookConfig struct {
	Agent         string
	Shape         HookShape
	Format        ConfigFormat
	Events        []HookRegistration
	CommandKeys   []string
	EventArgs     []string
	Defaults      json.RawMessage            // root fields added only when absent
	ContainerPath []string                   // nil: hooks, or file root for RootHooks
	EnablePath    []string                   // set true only while explicitly installing a non-optional hook
	OwnsHandler   func(json.RawMessage) bool // optional managed transport, e.g. ExitCodeBridge.Owns
}

// OwnedHookCommand recognises a simple native/PowerShell invocation, optionally
// with one --event argument. Unknown flags, relative paths and compound shell
// expressions confer no ownership.
func OwnedHookCommand(command, agent string, events []string) bool {
	_, ok := ownedHookCommandArgs(command, agent, events)
	return ok
}

func ownedHookCommandArgs(command, agent string, events []string) ([]string, bool) {
	// shellWords is intentionally conservative; reject interpolation even in
	// double quotes because the executed command could differ from its text.
	if strings.ContainsAny(command, "`$\x00\n\r") {
		return nil, false
	}
	command = strings.TrimSpace(command)
	if strings.HasPrefix(command, "& ") {
		command = strings.TrimSpace(command[2:])
	}
	words, ok := shellWords(command)
	if !ok || len(words) < 3 || !ownedHookArgs(words[1:], agent, events) {
		return nil, false
	}
	exe := strings.ReplaceAll(words[0], `\`, "/")
	base := exe[strings.LastIndex(exe, "/")+1:]
	if !strings.EqualFold(base, "obit") && !strings.EqualFold(base, "obit.exe") {
		return nil, false
	}
	if !filepath.IsAbs(exe) && !wslHookPath(exe) {
		return nil, false
	}
	return words[1:], true
}

func wslHookPath(exe string) bool {
	return strings.HasPrefix(exe, "/mnt/") && len(exe) > 7 && exe[5] >= 'a' && exe[5] <= 'z' && exe[6] == '/'
}

// OwnedHookExec recognises argv execution, never shell source or PATH lookup.
// An older absolute executable location remains owned so moves can be refreshed.
func OwnedHookExec(command string, args []string, agent string, events []string) bool {
	base := filepath.Base(command)
	native := strings.EqualFold(base, "obit.exe") || runtime.GOOS != "windows" && base == "obit"
	if !filepath.IsAbs(command) || !native ||
		strings.ContainsAny(command, "\x00\r\n\"|<>*") || strings.HasSuffix(command, "/") || strings.HasSuffix(command, `\`) {
		return false
	}
	if _, err := pathutil.Abs(command); err != nil {
		return false
	}
	return ownedHookArgs(args, agent, events)
}

func ownedHookArgs(args []string, agent string, events []string) bool {
	if agent == "" || len(args) < 2 || args[0] != "hook" || args[1] != agent {
		return false
	}
	switch {
	case len(args) == 2:
		return true
	case len(args) == 3 && args[2] == "--notice-only":
		return agent == "claude" || agent == "codex"
	case len(args) == 4 && args[2] == "--event":
		return slices.Contains(events, args[3]) && args[3] != ""
	case len(args) == 3 && strings.HasPrefix(args[2], "--event="):
		event := strings.TrimPrefix(args[2], "--event=")
		return event != "" && slices.Contains(events, event)
	default:
		return false
	}
}

func (spec HookConfig) owns(raw json.RawMessage) bool {
	if spec.OwnsHandler != nil && spec.OwnsHandler(raw) {
		return true
	}
	entry, err := parseObject(raw)
	if err != nil {
		return false
	}
	argsRaw, execForm := entry.get("args")
	if kind, exists := entry.get("type"); exists && !sameConfigJSON(kind, []byte(`"command"`)) &&
		(!execForm || !sameConfigJSON(kind, []byte(`"process"`))) {
		return false
	}
	var args []string
	if execForm && (!bytes.HasPrefix(bytes.TrimSpace(argsRaw), []byte("[")) || json.Unmarshal(argsRaw, &args) != nil) {
		return false
	}
	keys := spec.CommandKeys
	if len(keys) == 0 {
		keys = []string{"command"}
	}
	// An alternate launcher, shell flag or differently cased argv member can
	// change execution. It must not be silently ignored during ownership checks.
	for _, key := range entry.keys {
		lower := strings.ToLower(key)
		if lower == "args" && key != "args" || lower == "type" && key != "type" ||
			(lower == "command" || lower == "commandwindows" || lower == "bash" || lower == "powershell") && !slices.Contains(keys, key) {
			return false
		}
	}
	if execForm {
		if shell, ok := entry.get("shell"); ok && !sameConfigJSON(shell, []byte("false")) {
			return false
		}
	}
	found := false
	for _, key := range keys {
		if raw, exists := entry.get(key); exists {
			var command string
			if json.Unmarshal(raw, &command) != nil {
				return false
			}
			if command == "" {
				continue
			}
			if !spec.ownsCommand(command, args, execForm) {
				return false
			}
			found = true
		}
	}
	return found
}

// PlanHookConfig merges, refreshes and precisely removes native JSON hooks.
// JSONC uses the same semantics with AST edits that retain comments.
func PlanHookConfig(s Scope, path string, spec HookConfig, uninstall bool) (Change, error) {
	ch, err := readIntegrationChange(s, path)
	if err != nil {
		return ch, err
	}
	if spec.ContainerPath != nil {
		if err := nonemptyConfig(ch); err != nil {
			return ch, err
		}
	}
	if spec.Format == "" {
		spec.Format = ConfigJSON
	}
	if spec.Shape != GroupedHooks && spec.Shape != FlatHooks && spec.Shape != RootHooks {
		return ch, fmt.Errorf("unknown hook shape %q", spec.Shape)
	}
	root, hooks, prefix, err := hookContainer(ch, spec)
	if err != nil {
		return ch, err
	}
	edits, seen, err := hookEventEdits(path, hooks, prefix, spec, uninstall)
	if err != nil {
		return ch, err
	}
	if len(edits) > 0 && len(prefix) > 0 && hooks.empty() {
		edits = []ConfigEdit{{Path: prefix, Remove: true}}
	}
	edits, err = hookDefaultEdits(edits, root, prefix, seen, spec, uninstall)
	if err != nil {
		return ch, err
	}
	return planHookEdits(ch, spec, edits, uninstall)
}

func hookContainer(ch Change, spec HookConfig) (*object, *object, []string, error) {
	raw, err := configJSON(ch.orig, spec.Format)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%s: %w", ch.Path, err)
	}
	root, err := parseObject(raw)
	if err != nil {
		return nil, nil, nil, err
	}
	prefix := slices.Clone(spec.ContainerPath)
	if prefix == nil && spec.Shape != RootHooks {
		prefix = []string{"hooks"}
	}
	hooks := root
	for _, member := range prefix {
		if member == "" {
			return nil, nil, nil, fmt.Errorf("empty hook container member")
		}
		hooks, err = parseObject(hooks.values[member])
		if err != nil {
			return nil, nil, nil, fmt.Errorf("%s: %s must be an object: %w", ch.Path, strings.Join(prefix, "."), err)
		}
	}
	return root, hooks, prefix, nil
}

func hookEventEdits(path string, hooks *object, prefix []string, spec HookConfig, uninstall bool) ([]ConfigEdit, map[string]bool, error) {
	var edits []ConfigEdit
	seen := map[string]bool{}
	for _, ev := range spec.Events {
		if ev.Event == "" || seen[ev.Event] || !uninstall && !spec.owns(ev.Handler) {
			return nil, nil, fmt.Errorf("invalid or duplicate %s hook registration for %s", ev.Event, spec.Agent)
		}
		seen[ev.Event] = true
		old, exists := hooks.get(ev.Event)
		var entries []json.RawMessage
		if exists && (!bytes.HasPrefix(bytes.TrimSpace(old), []byte("[")) || json.Unmarshal(old, &entries) != nil) {
			return nil, nil, fmt.Errorf("%s: event %s must be an array", path, ev.Event)
		}
		kept, changed, err := mergeHookEntries(entries, ev, spec, uninstall)
		if err != nil {
			return nil, nil, err
		}
		if !changed {
			continue
		}
		edit := ConfigEdit{Path: append(slices.Clone(prefix), ev.Event), Remove: len(kept) == 0}
		if !edit.Remove {
			edit.Value, err = marshal(kept)
			if err != nil {
				return nil, nil, fmt.Errorf("encode %s hooks: %w", ev.Event, err)
			}
		}
		edits = append(edits, edit)
		if edit.Remove {
			hooks.remove(ev.Event)
		} else {
			_ = hooks.set(ev.Event, edit.Value)
		}
	}
	return edits, seen, nil
}

func hookDefaultEdits(edits []ConfigEdit, root *object, prefix []string, seen map[string]bool, spec HookConfig, uninstall bool) ([]ConfigEdit, error) {
	if !uninstall && len(spec.EnablePath) > 0 && slices.ContainsFunc(spec.Events, func(ev HookRegistration) bool { return !ev.Optional }) {
		if len(prefix) == 0 || slices.Equal(spec.EnablePath[:min(len(prefix), len(spec.EnablePath))], prefix[:min(len(prefix), len(spec.EnablePath))]) {
			return nil, fmt.Errorf("hook enable member overlaps event container")
		}
		edits = append(edits, ConfigEdit{Path: spec.EnablePath, Value: json.RawMessage("true")})
	}
	if !uninstall && len(spec.Defaults) > 0 {
		defaults, err := parseObject(spec.Defaults)
		if err != nil {
			return nil, err
		}
		for _, key := range defaults.keys {
			if len(prefix) > 0 && key == prefix[0] || len(prefix) == 0 && seen[key] {
				return nil, fmt.Errorf("hook defaults overlap event container %s", key)
			}
			if _, exists := root.get(key); !exists {
				edits = append(edits, ConfigEdit{Path: []string{key}, Value: defaults.values[key]})
			}
		}
	}
	return edits, nil
}

func planHookEdits(ch Change, spec HookConfig, edits []ConfigEdit, uninstall bool) (Change, error) {
	out, err := editConfig(ch.orig, spec.Format, edits)
	if err != nil {
		return ch, err
	}
	if bytes.Equal(ch.orig, out) {
		return ch, nil
	}
	if uninstall && spec.Format == ConfigJSON && sameConfigJSON(bytes.TrimPrefix(out, []byte("\xef\xbb\xbf")), []byte("{}")) {
		ch.Action, ch.delete = "remove", true
		return ch, nil
	}
	ch.Detail = "Obituary hooks: " + spec.Agent
	return setIntegrationData(ch, out), nil
}

func mergeHookEntries(entries []json.RawMessage, ev HookRegistration, spec HookConfig, uninstall bool) ([]json.RawMessage, bool, error) {
	m := hookEntryMerge{spec: spec, event: ev, uninstall: uninstall}
	var kept []json.RawMessage
	for _, raw := range entries {
		kept = append(kept, m.merge(raw)...)
	}
	if !uninstall && !m.present && (!ev.Optional || m.relocate) {
		value := ev.Handler
		if spec.Shape != FlatHooks {
			group := newObject()
			if ev.Matcher != "" {
				_ = group.set("matcher", ev.Matcher)
			}
			_ = group.set("hooks", []json.RawMessage{ev.Handler})
			value, _ = group.MarshalJSON()
		}
		kept, m.changed = append(kept, value), true
	}
	return kept, m.changed, nil
}

type hookEntryMerge struct {
	spec                       HookConfig
	event                      HookRegistration
	uninstall                  bool
	present, changed, relocate bool
}

func (m *hookEntryMerge) merge(raw json.RawMessage) []json.RawMessage {
	group, err := parseObject(raw)
	if err != nil {
		return []json.RawMessage{raw}
	}
	var handlers []json.RawMessage
	if m.spec.Shape == FlatHooks {
		handlers = []json.RawMessage{raw}
	} else {
		hooks, ok := group.get("hooks")
		if !ok || !bytes.HasPrefix(bytes.TrimSpace(hooks), []byte("[")) || json.Unmarshal(hooks, &handlers) != nil {
			return []json.RawMessage{raw}
		}
	}
	remaining, changed := m.mergeHandlers(group, handlers)
	m.changed = m.changed || changed
	switch {
	case !changed:
		return []json.RawMessage{raw}
	case m.spec.Shape == FlatHooks:
		return remaining
	case len(remaining) > 0:
		_ = group.set("hooks", remaining)
		value, _ := group.MarshalJSON()
		return []json.RawMessage{value}
	default:
		return emptyHookGroup(group)
	}
}

func (m *hookEntryMerge) mergeHandlers(group *object, handlers []json.RawMessage) ([]json.RawMessage, bool) {
	var remaining []json.RawMessage
	changed := false
	for _, h := range handlers {
		if !m.spec.owns(h) {
			remaining = append(remaining, h)
			continue
		}
		if m.spec.Shape != FlatHooks && !m.uninstall {
			var matcher string
			_ = json.Unmarshal(group.values["matcher"], &matcher)
			if matcher != m.event.Matcher {
				m.relocate, changed = true, true
				continue
			}
		}
		if m.uninstall || m.present {
			changed = true
			continue
		}
		m.present = true
		if !sameConfigJSON(h, m.event.Handler) {
			h, changed = m.event.Handler, true
		}
		remaining = append(remaining, h)
	}
	return remaining, changed
}

func emptyHookGroup(group *object) []json.RawMessage {
	// Preserve custom group metadata even after its last owned handler.
	for _, key := range group.keys {
		if key != "hooks" && key != "matcher" {
			_ = group.set("hooks", []json.RawMessage{})
			value, _ := group.MarshalJSON()
			return []json.RawMessage{value}
		}
	}
	return nil
}

func (spec HookConfig) ownsCommand(command string, args []string, execForm bool) bool {
	if execForm {
		return OwnedHookExec(command, args, spec.Agent, spec.EventArgs)
	}
	return OwnedHookCommand(command, spec.Agent, spec.EventArgs)
}
