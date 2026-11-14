package hook

import (
	"fmt"
	"strings"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/textutil"
)

// maxCleanupListed bounds the files one cleanup request names.
const maxCleanupListed = 10

type cleanupAck struct{ root, session string }

func listCleanup(files []auto.CleanupFile, root string) string {
	var parts []string
	for i, f := range files {
		if i == maxCleanupListed {
			parts = append(parts, fmt.Sprintf("and %d more", len(files)-maxCleanupListed))
			break
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", rel(root, f.Path), textutil.Bytes(f.Size)))
	}
	return strings.Join(parts, ", ")
}

// stopCleanup is the Stop hook's one request per session to an agent that
// can be asked to continue. The reason names only files this session's agent
// created; it is written as statements, like every other hook text.
func stopCleanup(e *auto.Engine, root, session string) (delivery, error) {
	if e.CleanupState(root, session) != "" {
		return delivery{}, nil
	}
	files, err := e.CleanupCandidates(root, session)
	if err != nil || len(files) == 0 {
		return delivery{}, err
	}
	// Claim the one request before returning it: a lost reply is better than a loop.
	if claimed, err := e.MarkCleanup(root, session, auto.CleanupAsked); err != nil || !claimed {
		return delivery{}, err
	}
	text := fmt.Sprintf("Obituary (讣告) session check, made once per session: this session created %s that %s no lifespan and that Git does not track: %s. "+
		"Scratch among them (debug scripts, logs, screenshots, temporary output) gets a lifespan with `obit mark <path> 2d`, or goes to the graveyard now with `obit bury <path>`; "+
		"deliverables stay where they are. Only the files listed here are in question.",
		textutil.Count(len(files), "file"), plural(len(files), "has", "have"), listCleanup(files, root))
	return delivery{lines: []string{text}, cont: true}, nil
}

// earlierCleanups tells a session start about sessions in the project whose
// agent could not be asked when it stopped. Each is told once.
func earlierCleanups(e *auto.Engine, root string, context *delivery) {
	for _, session := range e.PendingCleanups(root) {
		files, err := e.CleanupCandidates(root, session)
		if err != nil {
			Logf(e.Svc.Store.Home, "session cleanup for %q: %v", session, err)
			continue
		}
		if len(files) == 0 {
			if _, err := e.MarkCleanup(root, session, auto.CleanupTold); err != nil {
				Logf(e.Svc.Store.Home, "session cleanup for %q: %v", session, err)
			}
			continue
		}
		text := fmt.Sprintf("Obituary (讣告): an earlier agent session in this project created %s that still %s no lifespan and that Git does not track: %s. "+
			"Scratch among them (debug scripts, logs, screenshots, temporary output) can get a lifespan with `obit mark <path> 2d`; deliverables stay where they are. "+
			"Only the files listed here are in question.",
			textutil.Count(len(files), "file"), plural(len(files), "has", "have"), listCleanup(files, root))
		var fits bool
		if context.lines, fits = appendContext(context.lines, text); fits {
			context.cleanups = append(context.cleanups, cleanupAck{root: root, session: session})
		}
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
