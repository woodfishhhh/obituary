package integrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// sameJSON compares two JSON values ignoring formatting and key order.
func sameJSON(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	xb, _ := json.Marshal(x)
	yb, _ := json.Marshal(y)
	return string(xb) == string(yb)
}

// isObitHook uses the same absolute-path and argument checks as native hooks.
func isObitHook(command, agent string) bool {
	return OwnedHookCommand(command, agent, nil)
}

// shellWords splits a simple command line, honouring single and double
// quotes; anything with shell operators is not a simple command.
func shellWords(command string) ([]string, bool) {
	var words []string
	var cur strings.Builder
	inWord := false
	var quote rune
	for _, r := range command {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote, inWord = r, true
		case r == ' ' || r == '\t':
			if inWord {
				words, inWord = append(words, cur.String()), false
				cur.Reset()
			}
		case strings.ContainsRune("&|;<>()`$\n\r", r):
			return nil, false
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if quote != 0 {
		return nil, false
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words, true
}

type handler struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Timeout int      `json:"timeout,omitempty"`
}

// hookEvent is one registration: an event, its matcher and the handler JSON.
type hookEvent struct {
	Event, Matcher string
	Handler        any
	Optional       bool // refresh/remove if present; only add on explicit opt-in
}

// hookSpec describes an agent's hook file: what to register, and how to
// recognise handlers Obituary owns.
type hookSpec struct {
	Events       []hookEvent
	IsOurs       func(handler json.RawMessage) bool
	PreserveNote func(handler json.RawMessage) string
}

// planHooks adds or removes Obituary's handlers in a Claude-style hook file
// (event, then matcher groups, then handlers), keeping everything else.
//
//nolint:funlen // Existing planner body; only the inspection scope is passed through here.
func planHooks(s Scope, path string, spec hookSpec, uninstall bool) (ch Change, err error) {
	preserveNote := ""
	defer func() {
		if preserveNote != "" {
			ch.Detail = strings.TrimSpace(ch.Detail + " " + preserveNote)
		}
	}()
	ch, err = readIntegrationChange(s, path)
	if err != nil {
		return ch, err
	}
	data, exists := ch.orig, ch.existed
	root, err := parseObject(data)
	if err != nil {
		return ch, fmt.Errorf("%s is not valid JSON (left untouched): %w", path, err)
	}
	hooks := newObject()
	if raw, ok := root.get("hooks"); ok {
		if hooks, err = parseObject(raw); err != nil {
			return ch, fmt.Errorf("%s: \"hooks\" is not an object (left untouched): %w", path, err)
		}
	}
	changed := false
	var notes []string
	for _, ev := range spec.Events {
		var groups []json.RawMessage
		if raw, ok := hooks.get(ev.Event); ok {
			if err := json.Unmarshal(raw, &groups); err != nil {
				return ch, fmt.Errorf("%s: hooks.%s is not an array (left untouched)", path, ev.Event)
			}
		}
		present, eventChanged := false, false
		var kept []json.RawMessage
		for _, g := range groups {
			group, err := parseObject(g)
			if err != nil {
				kept = append(kept, g) // not ours to judge; keep as is
				continue
			}
			var handlers []json.RawMessage
			if raw, ok := group.get("hooks"); ok {
				_ = json.Unmarshal(raw, &handlers)
			}
			var keepHandlers []json.RawMessage
			groupChanged := false
			for _, h := range handlers {
				if !spec.IsOurs(h) {
					if spec.PreserveNote != nil {
						if note := spec.PreserveNote(h); note != "" {
							preserveNote = note
						}
					}
					keepHandlers = append(keepHandlers, h)
					continue
				}
				var matcher string
				if raw, ok := group.get("matcher"); ok {
					_ = json.Unmarshal(raw, &matcher)
				}
				if ev.Event == "PreToolUse" && matcher != ev.Matcher && !uninstall {
					eventChanged, groupChanged = true, true
					ev.Optional = false // relocate our handler; leave others in place
					continue
				}
				switch {
				case uninstall:
					eventChanged, groupChanged = true, true
					continue
				case present:
					// a duplicate of ours (would run twice): drop it
					eventChanged, groupChanged = true, true
					continue
				}
				present = true
				if want, _ := marshal(ev.Handler); !sameJSON(h, want) {
					// ours, but stale (moved binary, new timeout): refresh in place
					h = want
					eventChanged, groupChanged = true, true
					notes = append(notes, "refresh "+ev.Event)
				}
				keepHandlers = append(keepHandlers, h)
			}
			switch {
			case !groupChanged:
				kept = append(kept, g)
			case len(keepHandlers) == 0:
				// the group only held Obituary's handler: drop it
			default:
				b, _ := marshal(keepHandlers)
				_ = group.set("hooks", json.RawMessage(b))
				ob, _ := group.MarshalJSON()
				kept = append(kept, ob)
			}
		}
		if !uninstall && !present && !ev.Optional {
			group := newObject()
			if ev.Matcher != "" {
				_ = group.set("matcher", ev.Matcher)
			}
			_ = group.set("hooks", []any{ev.Handler})
			gb, _ := group.MarshalJSON()
			kept = append(kept, gb)
			eventChanged = true
			notes = append(notes, "add "+ev.Event)
		}
		if !eventChanged {
			continue
		}
		changed = true
		if len(kept) == 0 {
			hooks.remove(ev.Event)
			continue
		}
		b, _ := marshal(kept)
		if err := hooks.set(ev.Event, json.RawMessage(b)); err != nil {
			return ch, err
		}
	}
	if !changed {
		return ch, nil
	}
	if hooks.empty() {
		root.remove("hooks")
	} else if err := root.set("hooks", hooks); err != nil {
		return ch, err
	}
	if uninstall && root.empty() {
		ch.Action, ch.Detail, ch.delete = "remove", "only held Obituary's hooks", true
		return ch, nil
	}
	out, err := root.pretty()
	if err != nil {
		return ch, err
	}
	if sameJSON(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), out) {
		return ch, nil
	}
	if bytes.HasPrefix(data, []byte("\xef\xbb\xbf")) {
		out = append([]byte("\xef\xbb\xbf"), out...)
	}
	ch.data = out
	switch {
	case uninstall:
		ch.Action, ch.Detail = "update", "remove Obituary's hooks"
	case exists:
		ch.Action, ch.Detail = "update", "hooks: "+strings.Join(notes, ", ")
	default:
		ch.Action, ch.Detail = "create", "hooks: "+strings.Join(notes, ", ")
	}
	return ch, nil
}

// guardHook requires every platform invocation to opt into deletion checks.
// Refreshing a mixed-mode handler must preserve the advisory-only platform.
func guardHook(raw json.RawMessage, agent string) bool {
	spec := HookConfig{Agent: agent, CommandKeys: []string{"command"}}
	if agent == "codex" {
		spec = codexHookConfig()
	}
	if !spec.owns(raw) {
		return false
	}
	var entry map[string]json.RawMessage
	if json.Unmarshal(raw, &entry) != nil {
		return false
	}
	var args []string
	if value, execForm := entry["args"]; execForm {
		return json.Unmarshal(value, &args) == nil && len(args) == 2
	}
	for _, key := range spec.CommandKeys {
		var command string
		_ = json.Unmarshal(entry[key], &command)
		if command == "" {
			continue
		}
		args, ok := ownedHookCommandArgs(command, agent, nil)
		if !ok || len(args) != 2 {
			return false
		}
	}
	return true
}

// GuardRMInstalled reads the original hook file in a plan, not its proposed
// contents; --show must report the actual state even with --guard-rm supplied.
func GuardRMInstalled(changes []Change, agent string) bool {
	_, effective := GuardRMStatus(changes, agent)
	return effective
}

// GuardRMStatus distinguishes a registered handler from coverage of every shell
// tool handled by that agent. Matchers in separate groups contribute together.
func GuardRMStatus(changes []Change, agent string) (registered, effective bool) {
	tools := []string{"Bash", "PowerShell"}
	if agent == "codex" {
		tools = []string{"Bash", "exec_command", "shell_command", "shell", "local_shell"}
	}
	covered := make(map[string]bool)
	for _, ch := range changes {
		root, err := parseObject(ch.orig)
		if err != nil {
			continue
		}
		var hooks map[string][]struct {
			Matcher string            `json:"matcher"`
			Hooks   []json.RawMessage `json:"hooks"`
		}
		if json.Unmarshal(root.values["hooks"], &hooks) != nil {
			continue
		}
		for _, group := range hooks["PreToolUse"] {
			for _, raw := range group.Hooks {
				if guardHook(raw, agent) {
					registered = true
					pattern := group.Matcher
					if pattern == "" || pattern == "*" {
						pattern = ".*"
					}
					matcher, err := regexp.Compile(pattern)
					if err != nil {
						continue
					}
					for _, tool := range tools {
						if matcher.MatchString(tool) {
							covered[tool] = true
						}
					}
				}
			}
		}
	}
	return registered, len(covered) == len(tools)
}
