package docker

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// ListIDs reads the inventory on the pinned daemon without resolving IDs as names.
func (c CLI) ListIDs(ctx context.Context) ([]string, error) {
	bound, err := c.bind(ctx)
	if err != nil {
		return nil, err
	}
	return bound.listIDs(ctx)
}

func (c CLI) listIDs(ctx context.Context) ([]string, error) {
	out, err := c.run(ctx, "ps", "--all", "--no-trunc", "--format", "{{json .ID}}")
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var id string
		if err := json.Unmarshal([]byte(line), &id); err != nil {
			return nil, err
		}
		decoded, err := hex.DecodeString(id)
		if err != nil || len(decoded) != 32 || id != strings.ToLower(id) {
			return nil, ErrIdentity
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// CheckRemoval detects collateral disappearance; CLI inventories cannot establish
// causality or include a replacement created and removed between both snapshots.
func CheckRemoval(before, after []string, target string) error {
	present := make(map[string]bool, len(after))
	for _, id := range after {
		present[id] = true
	}
	for _, id := range before {
		if id != target && !present[id] {
			return fmt.Errorf("%w: another container disappeared during rm: %s", ErrAmbiguousAction, id)
		}
	}
	return nil
}
