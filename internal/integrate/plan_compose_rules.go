package integrate

import (
	"bytes"
	"fmt"
)

// Agent-specific Markdown planners already validate ownership markers. Reuse
// the conservative byte-range composer for disjoint blocks and identical shared
// blocks; never parse/reformat the user's Markdown or guess through overlaps.
func composeRuleChanges(changes []Change) (Change, error) {
	first := changes[0]
	var edits []composeTextEdit
	var removal bool
	for _, ch := range changes {
		if ch.existed != first.existed || !bytes.Equal(ch.orig, first.orig) || ch.project != first.project {
			return first, fmt.Errorf("rule plans have different original content or boundaries")
		}
		if ch.next != nil || ch.claude != nil || len(ch.Commands) != 0 || ch.Action == "preserve" {
			return first, fmt.Errorf("opaque rule plans cannot be combined")
		}
		first = mergeChangeMetadata(first, ch)
		if ch.Action == "unchanged" {
			continue
		}
		if ch.delete {
			removal = true
			continue
		}
		edit := composeTextDiff(ch.orig, ch.data)
		duplicate := false
		for _, prior := range edits {
			if edit.start == prior.start && edit.end == prior.end && bytes.Equal(edit.data, prior.data) {
				duplicate = true
			}
		}
		if !duplicate {
			edits = append(edits, edit)
		}
	}
	if removal {
		if len(edits) != 0 {
			return first, fmt.Errorf("whole-file rule removal conflicts with another agent's block")
		}
		first.Action, first.delete = "remove", true
		return first, nil
	}
	data, err := composeTOMLText(first.orig, edits)
	if err != nil {
		return first, fmt.Errorf("overlapping rule edits: %w", err)
	}
	first.Action, first.data = "unchanged", nil
	first = setIntegrationData(first, data)
	first.Detail = "combined Obituary rule blocks; surrounding user text retained"
	return first, nil
}
