package integrate

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// Scope selects user-wide or project configuration.
type Scope struct {
	Global     bool
	Project    string // project root for the non-global scope
	GuardRM    bool   // opt-in; an existing guard is retained by ordinary refreshes
	MCP        bool   // opt-in stdio registration
	Skill      bool   // opt-in Agent Skill
	inspection *integrationInspection
}

// claudePaths returns settings.json, the OBIT.md awareness file, the
// CLAUDE.md that references it and the reference line itself.
func claudePaths(s Scope) (settings, awareness, claudeMD, ref string, err error) {
	if s.Global {
		dir, err := userdirs.ClaudeDir()
		if err != nil {
			return "", "", "", "", err
		}
		return filepath.Join(dir, "settings.json"), filepath.Join(dir, "OBIT.md"), filepath.Join(dir, "CLAUDE.md"), "@OBIT.md", nil
	}
	dir := filepath.Join(s.Project, ".claude")
	settings, awareness, claudeMD = filepath.Join(dir, "settings.json"), filepath.Join(dir, "OBIT.md"), filepath.Join(s.Project, "CLAUDE.md")
	for _, p := range []string{settings, awareness, claudeMD} {
		if err := s.checkInside(s.Project, p, p+".bak"); err != nil {
			return "", "", "", "", err
		}
	}
	return settings, awareness, claudeMD, "@.claude/OBIT.md", nil
}

// PlanClaude computes the changes for installing (or, with uninstall,
// removing) the Claude Code integration. Nothing is written.
func PlanClaude(s Scope, command string, uninstall bool) ([]Change, error) {
	settingsPath, awarenessPath, claudeMDPath, ref, err := claudePaths(s)
	if err != nil {
		return nil, err
	}
	var changes []Change
	spec := claudeSpec(command)
	noticeCommand, err := graveNoticeCommand(s, settingsPath, "claude", command)
	if err != nil {
		return nil, err
	}
	spec.Events = append(spec.Events, hookEvent{Event: "PreToolUse", Matcher: ".*", Handler: claudeHandler(noticeCommand)})
	shared, err := claudeHookConfig(spec)
	if err != nil {
		return nil, err
	}
	c, err := PlanHookConfig(s, settingsPath, shared, uninstall)
	if err != nil {
		return nil, err
	}
	changes = append(changes, c)
	c, err = planFile(s, awarenessPath, []byte(Awareness), shippedAwareness, uninstall)
	if err != nil {
		return nil, err
	}
	changes = append(changes, c)
	c, err = planReference(s, claudeMDPath, ref, uninstall)
	if err != nil {
		return nil, err
	}
	changes = append(changes, c)
	changes[0].phase, changes[1].phase, changes[2].phase = 1, 0, 2
	if uninstall {
		for i := range changes {
			changes[i].phase = 2 - changes[i].phase
		}
	}
	references, err := planClaudeReference(s, changes[2], uninstall)
	if err != nil {
		return nil, err
	}
	// Keep the existing dependency order: import last on install, first on
	// removal. The managed sidecar persists ownership until removal commits.
	for i := range references {
		if references[i].Path == claudeMDPath {
			references[i].phase = changes[2].phase
		}
	}
	changes = append(changes[:2], references...)
	return changes, nil
}

func claudeHookConfig(spec hookSpec) (HookConfig, error) {
	shared := HookConfig{Agent: "claude", Shape: GroupedHooks}
	for _, ev := range spec.Events {
		raw, err := marshal(ev.Handler)
		if err != nil {
			return shared, err
		}
		shared.Events = append(shared.Events, HookRegistration{Event: ev.Event, Matcher: ev.Matcher, Handler: raw, Optional: ev.Optional})
	}
	return shared, nil
}

// PlanClaudeExec pins the executable even when PATH already resolves to obit.
// Claude Code >=2.1.139 executes command+args without choosing a shell (#12).
func PlanClaudeExec(s Scope, executable string, uninstall bool) ([]Change, error) {
	if !uninstall && !OwnedHookExec(executable, []string{"hook", "claude"}, "claude", nil) {
		return nil, fmt.Errorf("hooks for Claude require an absolute Obituary executable path")
	}
	return PlanClaude(s, fmt.Sprintf("\"%s\" hook claude", filepath.ToSlash(executable)), uninstall)
}

func claudeHandler(command string) handler {
	h := handler{Type: "command", Command: command}
	if words, ok := shellWords(command); ok && len(words) >= 3 && OwnedHookExec(words[0], words[1:], "claude", nil) {
		h.Command, h.Args = words[0], words[1:]
	}
	return h
}

func claudeSpec(command string) hookSpec {
	var events []hookEvent
	for _, ev := range hook.ClaudeEvents {
		h := claudeHandler(command)
		if ev.Event == "SessionEnd" {
			h.Timeout = 10 // SessionEnd hooks get 1.5 s unless they ask for more
		}
		events = append(events, hookEvent{Event: ev.Event, Matcher: ev.Matcher, Handler: h})
	}
	return hookSpec{Events: events, IsOurs: func(raw json.RawMessage) bool {
		return (HookConfig{Agent: "claude"}).owns(raw)
	}}
}
