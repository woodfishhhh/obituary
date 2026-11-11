package hook

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
	"github.com/woodfishhhh/obituary/internal/project"
)

// Runner hands slow work to background processes so a hook returns at once
// (our SessionEnd registration requests 3 s for Codex and 10 s for Claude;
// Claude's default is 1.5 s and an external environment override can change it).
// A nil Runner, or a nil field, does that work inline, as tests do.
type Runner struct {
	// Tick starts a one-shot round of reaping and purging when work is due.
	Tick func()
	// Finish computes leftovers away from the agent; final bypasses debounce.
	Finish func(root, session string, final bool)
}

// maxContext caps the UTF-8 bytes a hook adds to the agent's context: Obituary must
// never flood a conversation.
const maxContext = 2000

func capContext(text string) string {
	if len(text) <= maxContext {
		return text
	}
	const suffix = " … (`obit proposals` and `obit list` show everything)"
	end := maxContext - len(suffix)
	for !utf8.RuneStart(text[end]) {
		end--
	}
	cut := text[:end]
	if i := strings.LastIndex(cut, "\n"); i > maxContext/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,;\n") + suffix
}

// Event is an agent-neutral hook event; each adapter translates its agent's
// payload into one.
type Event struct {
	References    []string      // paths read, written, edited or listed by a tool
	Writing       bool          // a referenced path is about to be recreated/edited
	NoticeOnly    bool          // installation requests advisory context, never deletion denial
	Host          string        // trusted adapter name, assigned by RunEvent
	HostEvent     string        // original host event name, when different from Name
	Tool          string        // original host tool name
	DeferContext  bool          // observe/snapshot normally, leaving proposals unacknowledged
	Name          string        // PreToolUse, SessionStart, PostToolUse, Context, Stop, SessionEnd
	Session       string        // the agent's session id
	Cwd           string        // working directory (native path)
	Source        string        // SessionStart: startup, resume, ...
	Paths         []string      // PostToolUse: files the tool wrote (absolute or relative to Cwd)
	Command       string        // PreToolUse: original shell source, used only for suggestions
	Shell         guardrm.Shell // PreToolUse: syntax supplied by the tool context
	ShellDeclared bool          // shell came from owner config, not per-call evidence
	SkipReason    string        // unsupported tool envelope
}

// Handle carries out one event and returns the context the agent should see,
// capped at maxContext. With OBIT_HOOK_AUDIT=1 every event is also written
// to hook-audit.log.
func Handle(e *auto.Engine, ev Event, runner *Runner) (string, error) {
	context, err := prepareEvent(e, ev, runner, true)
	defer context.close()
	if err != nil {
		return "", err
	}
	context.acknowledge(ev.Session)
	return context.text(), nil
}

// RunEvent acknowledges only after writing the host response. Handle's callers
// receive the context directly and retain its original acknowledgement contract.
func prepareEvent(e *auto.Engine, ev Event, runner *Runner, pendingNotices bool) (delivery, error) {
	if ev.Name == "PreToolUse" {
		if !ev.NoticeOnly {
			text, err := guardDelete(e.Svc.Store.Home, ev)
			if text != "" || err != nil {
				return delivery{lines: []string{text}, deny: text != ""}, err
			}
		}
		var context delivery
		prepareGraveNotices(e.Svc.Store, ev, &context)
		return context, nil
	}
	// Share one lock-wait budget across context reads and registration writes;
	// a later wait must not restart the clock after an earlier one timed out.
	engine, svc := *e, *e.Svc
	svc.Store = svc.Store.WithDeadline(time.Now().Add(time.Second))
	engine.Svc = &svc
	e = &engine
	context, err := handle(e, ev, runner, pendingNotices)
	context.svc = e.Svc
	if err == nil && !context.skipped {
		prepareGraveNotices(e.Svc.Store, ev, &context)
	}
	if Auditing() {
		Auditf(e.Svc.Store.Home, "%s session=%s cwd=%s paths=%d context=%d err=%v", ev.Name, ev.Session, ev.Cwd, len(ev.Paths), len(context.text()), err)
	}
	return context, err
}

var findProject = project.Find

func handle(e *auto.Engine, ev Event, runner *Runner, pendingNotices bool) (delivery, error) {
	if runner == nil {
		runner = &Runner{}
	}
	if ev.Cwd == "" || ev.Session == "" {
		return delivery{}, fmt.Errorf("payload lacks cwd or session id")
	}
	if remoteWorkspace(ev.Cwd) {
		// A remote session's absolute paths name files on the remote host,
		// even when a local file happens to have the same path.
		return delivery{}, nil
	}
	proj, inProject, err := findProject(ev.Cwd)
	if err != nil && ev.Name != "PostToolUse" {
		if skipDiscoveryDenied(err) {
			// An unresolved project must never enter the non-project snapshot path.
			Logf(e.Svc.Store.Home, "skip: project discovery unavailable; event=%q cwd=%q reason=%q", ev.Name, ev.Cwd, err)
			return delivery{skipped: true}, nil
		}
		return delivery{}, fmt.Errorf("discover project from %q: %w", ev.Cwd, err)
	}
	switch ev.Name {
	case "SessionStart":
		return startSession(e, ev, runner, proj, inProject)
	case "PostToolUseFailure":
		return delivery{}, nil
	case "PostToolUse", "Context":
		return postToolContext(e, ev, runner, pendingNotices, proj, inProject)
	case "Stop", "SessionEnd":
		return finishSession(e, ev, runner, proj, inProject)
	}
	return delivery{}, nil
}

func startSession(e *auto.Engine, ev Event, runner *Runner, proj project.Project, inProject bool) (delivery, error) {
	replayPending(e)
	if runner.Tick != nil {
		runner.Tick()
	}
	if !inProject {
		return delivery{}, e.SnapshotSession(ev.Cwd, ev.Session)
	}
	if err := e.Snapshot(proj.Root, ev.Session); err != nil {
		return delivery{}, fmt.Errorf("snapshot project %q: %w", proj.Root, err)
	}
	if ev.DeferContext {
		return delivery{}, nil
	}
	return sessionContext(e, proj.Root, ev.Session)
}

func eventWritePaths(ev Event) []string {
	var paths []string
	for _, p := range ev.Paths {
		if ev.Name == "Context" {
			break // context callbacks never imply a successful write
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(ev.Cwd, p)
		}
		paths = append(paths, p)
	}
	return paths
}

func captureEventObservations(e *auto.Engine, ev Event, proj project.Project, paths []string) ([]core.Observation, error) {
	observations, captureErr := e.Svc.CaptureObservations(paths, ev.Session)
	for i := range observations {
		observations[i].Host, observations[i].Event = ev.Host, ev.Name
		if ev.HostEvent != "" {
			observations[i].Event = ev.HostEvent
		}
		observations[i].Project = proj.Root
		if target, ok, err := project.Find(filepath.Dir(observations[i].Path)); err == nil && ok {
			observations[i].Project = target.Root
		}
	}
	return observations, captureErr
}

func postToolContext(e *auto.Engine, ev Event, runner *Runner, pendingNotices bool, proj project.Project, inProject bool) (delivery, error) {
	paths := eventWritePaths(ev)
	observations, captureErr := captureEventObservations(e, ev, proj, paths)
	replayPending(e)
	if len(paths) == 0 && !pendingNotices && ev.Name != "Context" {
		return delivery{}, nil
	}
	decisions, notices, err := observeAvailable(e, observations)
	if err != nil {
		var pending []core.Observation
		for i, d := range decisions {
			if d.Action != auto.Retry {
				continue
			}
			pending = append(pending, observations[i])
		}
		err = errors.Join(err, e.Svc.Store.DeferObservations(pending))
		if len(pending) > 0 && runner.Tick != nil {
			runner.Tick()
		}
		return delivery{}, errors.Join(err, captureErr)
	}
	if captureErr != nil {
		return delivery{}, captureErr
	}
	if ev.DeferContext {
		return delivery{}, nil
	}
	return observationContext(e, ev, proj, inProject, pendingNotices, notices)
}

func observationContext(e *auto.Engine, ev Event, proj project.Project, inProject, pendingNotices bool, notices []auto.Notice) (delivery, error) {
	// A callback need not write another matching file to deliver earlier
	// observations. The ledger is the queue, scoped to the current project
	// or proposals created by this session (which may span projects).
	if pendingNotices || ev.Name == "Context" {
		state, err := e.Svc.Store.ReadHot()
		if err != nil {
			return delivery{}, err
		}
		for _, p := range core.ExistingProposals(state.Proposals) {
			if (p.Session == ev.Session || inProject && core.ProjectKey(p.Project) == core.ProjectKey(proj.Root)) &&
				!slices.Contains(p.Notified, ev.Session) && !slices.ContainsFunc(notices, func(n auto.Notice) bool { return n.Proposal.ID == p.ID }) {
				notices = append(notices, auto.Notice{Proposal: p})
			}
		}
	}
	var context delivery
	for _, n := range notices {
		if slices.Contains(n.Proposal.Notified, ev.Session) {
			continue
		}
		consent, err := proposalConsent(e.Svc, n.Proposal.Project)
		if err != nil {
			return delivery{}, err
		}
		context.append(n.Proposal, consent)
	}
	return context, nil
}

func finishSession(e *auto.Engine, ev Event, runner *Runner, proj project.Project, inProject bool) (delivery, error) {
	// Claude Stop additionalContext continues the turn. Codex Stop has
	// no additionalContext field; JSON continuation uses block + reason.
	// Both ending events stay silent; a runner computes the diff outside
	// the hook's time budget.
	if !inProject && runner.Finish != nil {
		runner.Finish(ev.Cwd, ev.Session, ev.Name == "SessionEnd")
	}
	if inProject {
		if runner.Finish != nil {
			runner.Finish(proj.Root, ev.Session, ev.Name == "SessionEnd")
		} else if _, err := e.FinishFiles(proj.Root, ev.Session); err != nil {
			return delivery{}, fmt.Errorf("finish project %q: %w", proj.Root, err)
		}
	}
	if ev.Name == "SessionEnd" && runner.Tick != nil {
		runner.Tick()
	}
	return delivery{}, nil
}

func replayPending(e *auto.Engine) {
	if !e.Svc.Store.HasDeferredObservations() {
		return
	}
	if err := e.Svc.Store.ProbeObservationStore(); err != nil {
		return
	}
	if err := e.ReplayObservations(); err != nil {
		Logf(e.Svc.Store.Home, "observation replay: %v", err)
	}
}

func observeAvailable(e *auto.Engine, rows []core.Observation) ([]auto.Decision, []auto.Notice, error) {
	if len(rows) > 0 {
		if err := e.Svc.Store.ProbeObservationStore(); err != nil {
			decisions := make([]auto.Decision, len(rows))
			for i, row := range rows {
				decisions[i] = auto.Decision{Path: row.Path, Action: auto.Retry, Err: err}
			}
			return decisions, nil, err
		}
	}
	return e.ObserveCaptured(rows)
}

// remoteWorkspace reports a cwd given as a URI (vscode-remote://, https://
// and the like) rather than a local path.
func remoteWorkspace(cwd string) bool {
	return strings.Contains(cwd, "://")
}
