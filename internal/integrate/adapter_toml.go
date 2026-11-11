package integrate

import (
	"fmt"
	"maps"
	"slices"

	"github.com/pelletier/go-toml/v2"
)

// Kimi and Vibe both use marked TOML hook arrays; only their event records,
// collision names and ownership predicates differ. Keep the existing TOML
// editor's exact refresh and marker validation.
type tomlHookAdapter struct {
	agent, id, detail, conflict string
	events                      []map[string]any
	reservedNames               []string
	owns                        func(map[string]any) bool
}

func (a tomlHookAdapter) plan(s Scope, path, command string, uninstall bool) (Change, error) {
	before, base, err := nativeSpecialTOMLBase(s, path, a.id)
	if err != nil {
		return Change{}, err
	}
	if err := a.checkUnmarked(path, base); err != nil {
		return Change{}, err
	}
	events := make([]map[string]any, 0, len(a.events))
	for _, event := range a.events {
		event = maps.Clone(event)
		event["command"] = command
		events = append(events, event)
	}
	content, err := toml.Marshal(map[string]any{"hooks": events})
	if err != nil {
		return Change{}, err
	}
	ch, err := PlanTOMLBlock(s, path, TOMLBlock{ID: a.id, Content: content, Owns: a.owns,
		Refresh: func(block []byte) ([]byte, error) {
			return RefreshTOMLString(block, []string{"hooks"}, "command", command)
		}}, uninstall)
	if err != nil {
		return Change{}, err
	}
	if err := nativeSpecialOriginal(ch, before); err != nil {
		return Change{}, err
	}
	ch.Detail = a.detail
	return ch, nil
}

func (a tomlHookAdapter) checkUnmarked(path string, base map[string]any) error {
	hooks, exists := base["hooks"]
	if !exists {
		return nil
	}
	entries, ok := hooks.([]any)
	if !ok {
		return fmt.Errorf("%s: hooks must be an array", path)
	}
	for _, item := range entries {
		entry, ok := item.(map[string]any)
		command, _ := entry["command"].(string)
		name, _ := entry["name"].(string)
		if !ok || slices.Contains(a.reservedNames, name) || OwnedHookCommand(command, a.agent, nil) {
			return fmt.Errorf("%s: %s", path, a.conflict)
		}
	}
	return nil
}
