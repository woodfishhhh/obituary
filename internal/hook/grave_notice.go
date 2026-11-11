package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

// A hook's shared-lock budget is one second; optional advisories use only 25 ms.
const graveNoticeBudget = 25 * time.Millisecond

var readNoticeSnapshot = func(s core.Store) (core.State, error) { return s.ReadSnapshot() }

func restorableNotice(b core.Burial, now time.Time) bool {
	return b.PurgeIntent == nil && b.PurgeAt.After(now)
}

func prepareGraveNotices(store core.Store, ev Event, d *delivery) {
	if ev.Session == "" || ev.Cwd == "" || ev.Name == "Stop" || ev.Name == "SessionEnd" {
		return
	}
	if agent, ok := Lookup(ev.Host); ok && agent.Capabilities.GraveNotice.Level == Unsupported {
		return
	}
	paths := absentReferences(ev)
	paths = slices.DeleteFunc(paths, func(p string) bool { return !store.MightHaveBurial(p) })
	pending := readGraveSession(store.Home, ev)
	if len(paths) == 0 && (ev.Name == "PreToolUse" || len(pending.Pending) == 0) {
		return
	}
	path := graveSessionPath(store.Home, ev)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return
	}
	unlock, ok, err := core.TryLockExclusive(path + ".lock")
	if err != nil || !ok {
		return
	}
	d.graveUnlock = unlock
	pending = readGraveSession(store.Home, ev)
	st, err := readNoticeSnapshot(store.WithDeadline(time.Now().Add(graveNoticeBudget)))
	if err != nil {
		return
	} // lock contention never queues or logs an advisory
	notices := selectGraveNotices(st.Burials, paths, pending, ev)
	if ev.DeferContext || ev.Name == "PreToolUse" && !preGraveContext(ev.Host) {
		writeGraveSession(store.Home, ev, notices, nil)
		return
	}
	appendGraveNotices(d, ev, notices)
	d.graveHome, d.graveEvent = store.Home, ev
}

func appendGraveNotices(d *delivery, ev Event, notices []graveNotice) {
	for _, n := range notices {
		text := describeGraveNotice(ev.Cwd, n)
		if len(text) > maxContext {
			text = fmt.Sprintf("Obituary: referenced path belongs to grave #%d (buried %s, restorable until %s). `obit graveyard --here` shows its path; `obit resurrect %d` restores it.", n.ID, n.Burial.BuriedAt.Format(time.RFC3339), n.Burial.PurgeAt.Format(time.RFC3339), n.ID)
		}
		if lines, fits := appendContext(d.lines, text); fits {
			d.lines, d.graves = lines, append(d.graves, n.ID)
		}
	}
}

type graveNotice struct {
	ID      int64       `json:"id"`
	Path    string      `json:"path"`
	Writing bool        `json:"writing,omitempty"`
	Burial  core.Burial `json:"-"`
}

func selectGraveNotices(burials []core.Burial, paths []string, session graveSession, ev Event) []graveNotice {
	var notices []graveNotice
	for _, b := range burials {
		if !restorableNotice(b, time.Now()) || slices.Contains(session.Seen, b.ID) {
			continue
		}
		var n graveNotice
		for _, p := range paths {
			if core.BurialPathMatches(b.Path, b.IsDir, p) {
				n = graveNotice{ID: b.ID, Path: p, Writing: ev.Writing}
				break
			}
		}
		if n.ID == 0 {
			for _, p := range session.Pending {
				if p.ID == b.ID {
					n = p
					break
				}
			}
		}
		if n.ID == 0 {
			continue
		}
		if _, err := os.Lstat(n.Path); !os.IsNotExist(err) && !n.Writing {
			continue
		}
		n.Burial = b
		notices = append(notices, n)
	}
	slices.SortFunc(notices, func(a, b graveNotice) int { return b.Burial.BuriedAt.Compare(a.Burial.BuriedAt) })
	return notices
}

func describeGraveNotice(cwd string, n graveNotice) string {
	b := n.Burial
	subject := fmt.Sprintf("%q was buried as #%d", rel(cwd, n.Path), b.ID)
	if n.Writing {
		subject = fmt.Sprintf("%q has an old version in the graveyard as #%d; this write remains allowed", rel(cwd, n.Path), b.ID)
	}
	if filepath.Clean(n.Path) != filepath.Clean(b.Path) {
		subject += fmt.Sprintf("; resurrection restores the entire directory #%d %q", b.ID, rel(cwd, b.Path))
	}
	return fmt.Sprintf("Obituary: %s (buried %s, restorable until %s). `obit resurrect %d` restores it.", subject, b.BuriedAt.Format(time.RFC3339), b.PurgeAt.Format(time.RFC3339), b.ID)
}

// Codex rust-v0.167.0 hooks/src/events/pre_tool_use.rs and
// hooks/src/engine/output_parser.rs forward additionalContext to the model.
func preGraveContext(host string) bool { return host == "claude" || host == "codex" }

func graveNoticeSupport(agent Agent) Support {
	if preGraveContext(agent.Name) {
		return Support{Level: Supported, Detail: "PreToolUse additionalContext for file/read/edit/list inputs and conservative literal shell operands; SessionStart graveyard summary"}
	}
	if agent.Name == "vibe" {
		return Support{Level: Limited, Detail: "Missing verified post_tool write paths only; SessionStart and pre-tool path notices unsupported"}
	}
	if agent.Capabilities.Context.Level != Unsupported && agent.Capabilities.SessionStart.Level != Unsupported {
		if agent.Name == "cursor" || agent.Name == "deepseek-harness" || agent.Name == "openclaw" || agent.Name == "hermes" || agent.Name == "kilocode" {
			return Support{Level: Limited, Detail: "Verified missing tool paths wait for the Context capability's model-visible callback; observation-only SessionStart summary unsupported"}
		}
		if agent.Capabilities.WriteObservation.Level == Unsupported {
			return Support{Level: Limited, Detail: "SessionStart graveyard summary only; tool-path notices unsupported"}
		}
		return Support{Level: Limited, Detail: "SessionStart summary; missing verified PostToolUse write paths or confirmed shell operands; delivery follows the Context capability"}
	}
	return Support{Detail: "No verified grave-notice context delivery"}
}
