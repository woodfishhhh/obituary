package mcp

import (
	"crypto/sha256"
	"fmt"
	"slices"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

func mcpMemberKey(row any) string {
	switch r := row.(type) {
	case ledger.Item:
		if r.Container != nil {
			return fmt.Sprintf("%q:%s:%s", r.Path, r.Container.ID, r.Container.Created)
		}
		identity := ""
		if r.Identity != nil {
			identity = fmt.Sprintf("%d:%s", r.Identity.Serial, r.Identity.FileID)
		}
		return fmt.Sprintf("%q:%s", r.Path, identity)
	case ledger.Proposal:
		candidates := make([]string, len(r.Candidates))
		for i, c := range r.Candidates {
			candidates[i] = fmt.Sprintf("%q:%d", c.Path, c.Rev)
		}
		slices.Sort(candidates)
		return fmt.Sprintf("%q", candidates)
	}
	return ""
}

// Hash the complete membership, independent of row order and page bounds.
// This detects late lower IDs without pretending a cursor is a frozen snapshot.
func mcpVersionedResult(data map[string]any, keys []string, in mcpPageInput, err error) mcpReply {
	slices.Sort(keys)
	hash := sha256.New()
	for _, key := range keys {
		fmt.Fprintln(hash, key)
	}
	version := fmt.Sprintf("%x", hash.Sum(nil)[:8])
	data["version"] = version
	reply := mcpResult(data, err)
	if in.Version != "" && in.Version != version {
		data["changed"] = true
		reply.Warning = "The list changed during pagination; read again from the beginning for a complete view."
	}
	return reply
}
