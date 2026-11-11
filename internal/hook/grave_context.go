package hook

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func buriedBySession(b core.Burial, session string) bool {
	if b.Operation != nil {
		return b.Operation.Session == session || b.Operation.Session == "claude:"+session || b.Operation.Session == "codex:"+session
	}
	return b.Session != "" && (b.Session == session || b.Session == "claude:"+session || b.Session == "codex:"+session)
}

func burialInProject(b core.Burial, root string) bool {
	project := b.Project
	if project == "" && b.Operation != nil {
		project = b.Operation.Project
	}
	if project != "" {
		return core.ProjectKey(project) == core.ProjectKey(root)
	}
	// Explicit burial of an unregistered path has no registration project.
	return core.BurialPathMatches(root, true, b.Path)
}

func graveSessionContext(burials []core.Burial, root, session string) string {
	var selected []core.Burial
	for _, b := range burials {
		if burialInProject(b, root) && restorableNotice(b, time.Now()) && b.Source != "mktemp" && !buriedBySession(b, session) {
			selected = append(selected, b)
		}
	}
	if len(selected) == 0 {
		return ""
	}
	slices.SortFunc(selected, func(a, b core.Burial) int { return b.BuriedAt.Compare(a.BuriedAt) })
	var parts []string
	for i, b := range selected {
		if i == maxListed {
			parts = append(parts, fmt.Sprintf("and %d more", len(selected)-maxListed))
			break
		}
		parts = append(parts, fmt.Sprintf("#%d %q (buried %s; until %s)", b.ID, rel(root, b.Path), b.BuriedAt.Format(time.RFC3339), b.PurgeAt.Format(time.RFC3339)))
	}
	text := fmt.Sprintf("Obituary: %d restorable items from earlier sessions are in this project's graveyard: %s. `obit graveyard --here` lists them; `obit resurrect <id>` restores one.", len(selected), strings.Join(parts, ", "))
	if len(text) > maxContext/2 {
		text = fmt.Sprintf("Obituary: %d restorable items from earlier sessions are in this project's graveyard. `obit graveyard --here` lists them; `obit resurrect <id>` restores one.", len(selected))
	}
	return text
}
