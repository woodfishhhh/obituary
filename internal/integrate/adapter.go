package integrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
)

// Native adapters declare the host schema; the existing checked editors still
// own parsing, ownership and writes so every registration uses the same
// ownership checks and installation transaction.
type hookAdapter struct {
	directory    func(Scope) (string, error)
	file         string
	guardReason  string
	config       HookConfig
	removeEvents []HookRegistration
	guardEvent   string
	handler      func(Commands, string) (json.RawMessage, error)
	check        func(Change) error
	rulesFile    string
	projectRules bool
	finish       func(Scope, string, Change, bool) ([]Change, error)
}

func (a hookAdapter) plan(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	if err := rejectUnsupportedGuard(s, uninstall, a.guardReason); err != nil {
		return nil, err
	}
	dir, err := a.directory(s)
	if err != nil {
		return nil, err
	}
	spec, err := a.hooks(s, commands, uninstall)
	if err != nil {
		return nil, err
	}
	hooks, err := PlanHookConfig(s, filepath.Join(dir, a.file), spec, uninstall)
	if err != nil {
		return nil, err
	}
	if err := nonemptyConfig(hooks); err != nil {
		return nil, err
	}
	if a.check != nil {
		if err := a.check(hooks); err != nil {
			return nil, err
		}
	}
	if a.finish != nil {
		return a.finish(s, dir, hooks, uninstall)
	}
	rulesPath := filepath.Join(dir, a.rulesFile)
	if a.projectRules && !s.Global {
		rulesPath = filepath.Join(s.Project, a.rulesFile)
	}
	return planHookRules(s, []Change{hooks}, rulesPath, uninstall)
}

func (a hookAdapter) hooks(s Scope, commands Commands, uninstall bool) (HookConfig, error) {
	spec := a.config
	spec.Events = slices.Clone(spec.Events)
	if uninstall {
		spec.Events = append(spec.Events, a.removeEvents...)
	}
	// A nil EventArgs means the host has no --event command suffix.
	if spec.EventArgs != nil {
		spec.EventArgs = nil
		for _, ev := range spec.Events {
			spec.EventArgs = append(spec.EventArgs, ev.Event)
		}
	}
	for i := range spec.Events {
		ev := &spec.Events[i]
		handler, err := a.handler(commands, ev.Event)
		if err != nil {
			return HookConfig{}, err
		}
		ev.Handler = handler
		if ev.Event == a.guardEvent {
			ev.Optional = !s.GuardRM
		}
	}
	return spec, nil
}

func rejectUnsupportedGuard(s Scope, uninstall bool, reason string) error {
	if s.GuardRM && !uninstall && reason != "" {
		return fmt.Errorf("%s", reason)
	}
	return nil
}

func nonemptyConfig(ch Change) error {
	if ch.existed && len(bytes.TrimSpace(bytes.TrimPrefix(ch.orig, []byte("\xef\xbb\xbf")))) == 0 {
		return fmt.Errorf("%s is empty; left untouched", ch.Path)
	}
	return nil
}

func powershellHookCommand(command string) string {
	command = strings.TrimSpace(command)
	if !strings.HasPrefix(command, "& ") {
		command = "& " + command
	}
	return command
}

func nativeHookHandler(fields map[string]any, powershell bool) func(Commands, string) (json.RawMessage, error) {
	return func(commands Commands, _ string) (json.RawMessage, error) {
		handler := maps.Clone(fields)
		command := commands.Native
		if powershell {
			command = powershellHookCommand(command)
		}
		handler["command"] = command
		return marshal(handler)
	}
}

func hookVersion(ch Change, host string) error {
	data, err := configJSON(ch.orig, ConfigJSON)
	if err != nil {
		return err
	}
	version, err := configMember(data, []string{"version"})
	if err != nil || version != nil && !sameConfigJSON(version, []byte("1")) {
		return fmt.Errorf("%s: only %s hook version 1 is supported", ch.Path, host)
	}
	return nil
}

func planHookRules(s Scope, hooks []Change, path string, uninstall bool) ([]Change, error) {
	if !s.Global {
		if err := s.checkInside(s.Project, path, path+".bak"); err != nil {
			return nil, err
		}
	}
	rules, err := planBlock(s, path, Awareness, uninstall)
	if err != nil {
		return nil, err
	}
	if !s.Global {
		rules.project = s.Project
	}
	for i := range hooks {
		hooks[i].phase = 1
		if uninstall {
			hooks[i].phase = 0
		}
	}
	if uninstall {
		rules.phase = 1
	}
	return append(hooks, rules), nil
}

func planNonemptyMCP(s Scope, path string, spec MCPConfig, executable string, uninstall bool) ([]Change, error) {
	ch, err := PlanMCPConfig(s, path, spec, executable, uninstall)
	if err != nil {
		return nil, err
	}
	if err := nonemptyConfig(ch); err != nil {
		return nil, err
	}
	return []Change{ch}, nil
}

func checkPlannedOriginal(changes []Change, before Change) error {
	for _, ch := range changes {
		if ch.Path == before.Path {
			if err := nativeSpecialOriginal(ch, before); err != nil {
				return err
			}
		}
	}
	return nil
}
