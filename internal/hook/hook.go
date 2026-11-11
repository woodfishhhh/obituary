// Package hook turns agent hook events into Obituary actions.
// Every agent adapter is a thin translation of that agent's JSON; the
// decisions live in Handle. Hook errors fail open: no output is emitted and
// the error goes to hook.log. The opt-in delete guard can deliberately deny.
package hook

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/rules"
	"github.com/woodfishhhh/obituary/internal/textutil"
)

// Disabled reports whether OBIT_DISABLED turns hooks off.
func Disabled() bool {
	v := strings.ToLower(os.Getenv("OBIT_DISABLED"))
	return v == "1" || v == "true" || v == "yes"
}

// Logf appends one line to hook.log in the data directory (best effort,
// rotated at 512 KiB under a lock, so concurrent hooks cannot clobber the
// previous log).
func Logf(home, format string, args ...any) { appendLog(home, "hook.log", format, args...) }

// IsErrorLogLine excludes explicit skips and legacy discovery denials in
// protected system locations. A non-nil roots slice overrides the OS locations.
// Unrecognized records remain errors so real failures surface.
func IsErrorLogLine(line string, roots ...string) bool {
	_, message, _ := strings.Cut(line, " ")
	if strings.HasPrefix(message, "skip: ") {
		return false
	}
	if roots == nil {
		roots = protectedDiscoveryRoots()
	}
	return !legacyDiscoveryDenied(message, roots)
}

func legacyDiscoveryDenied(message string, roots []string) bool {
	_, payload, ok := strings.Cut(message, ": ")
	if !ok {
		return false
	}
	quoted, err := strconv.QuotedPrefix(payload)
	if err != nil {
		return false
	}
	text, err := strconv.Unquote(quoted)
	if err != nil || !strings.HasPrefix(text, "discover project from ") {
		return false
	}
	var diagnostic struct {
		Operation string `json:"operation"`
		Path      string `json:"path"`
	}
	_, raw, ok := strings.Cut(payload[len(quoted):], permissionMarker)
	if !ok || json.Unmarshal([]byte(raw), &diagnostic) != nil || diagnostic.Operation != "resolve project directory" || !inProtectedDiscoveryRoot(diagnostic.Path, roots) {
		return false
	}
	cwd, err := strconv.QuotedPrefix(strings.TrimPrefix(text, "discover project from "))
	prefix := "discover project from " + cwd + ": " + diagnostic.Operation + " " + diagnostic.Path + ": "
	if err != nil || !strings.HasPrefix(text, prefix) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(strings.TrimPrefix(text, prefix))) {
	case "access is denied.", "access is denied", "access denied", "permission denied", "拒绝访问。", "拒绝访问":
		return true
	}
	return false
}

// PermissionHint also explains the fail-open degradation when doctor can read
// the data directory but the hook account cannot write its log or ledger.
// PermissionReportHint requests evidence, without promising an installer fix.
const PermissionReportHint = "把讣告升级到最新版后，再出现时把这一行发给维护者"

const PermissionHint = "The hook continues without blocking the agent; affected work is skipped or deferred. " +
	"obit doctor under the same account diagnoses access. A writable OBIT_HOME must be explicitly configured and uses a separate ledger; no automatic fallback is created."

// LogEventError keeps diagnostics out of the host protocol. Errors go to
// hook.log because ending hooks must stay silent. Quote untrusted fields
// onto one line to prevent forged log entries.
func LogEventError(home, agent string, ev Event, err error) {
	name, sid := "unknown", "unknown"
	if account, lookupErr := user.Current(); lookupErr == nil {
		name, sid = account.Username, account.Uid
	}
	hint := ""
	if errors.Is(err, os.ErrPermission) {
		hint = "; " + PermissionHint + permissionMarker + permissionDetails(err, name, sid)
	}
	Logf(home, "%s %s: %q; session=%q cwd=%q home=%q user=%q sid=%q pid=%d%s",
		agent, ev.Name, err, ev.Session, ev.Cwd, home, name, sid, os.Getpid(), hint)
}

// Auditing reports whether OBIT_HOOK_AUDIT asks for an audit trail.
func Auditing() bool {
	v := strings.ToLower(os.Getenv("OBIT_HOOK_AUDIT"))
	return v == "1" || v == "true" || v == "yes"
}

// Auditf appends one line to hook-audit.log (see `obit hook audit`).
func Auditf(home, format string, args ...any) { appendLog(home, "hook-audit.log", format, args...) }

func appendLog(home, name, format string, args ...any) {
	if err := os.MkdirAll(home, 0o700); err != nil {
		return
	}
	path := filepath.Join(home, name)
	var release func()
	for deadline := time.Now().Add(150 * time.Millisecond); ; {
		var ok bool
		var err error
		release, ok, err = core.TryLockExclusive(path + ".lock")
		if err != nil {
			return
		}
		if ok {
			break
		}
		if time.Now().After(deadline) {
			return // logging must not consume the hook's execution budget
		}
		time.Sleep(10 * time.Millisecond)
	}
	defer release()
	if info, err := os.Stat(path); err == nil && info.Size() > 512<<10 {
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
}

const maxListed = 5

// Separate the fallible context read from notification acknowledgement.
var proposalConsent = (*core.Service).Consent

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func rel(root, path string) string {
	if r, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(r, "..") {
		return filepath.ToSlash(r)
	}
	return path
}

func listCandidates(p core.Proposal) string {
	var parts []string
	for i, c := range p.Candidates {
		if i == maxListed {
			parts = append(parts, fmt.Sprintf("and %d more", len(p.Candidates)-maxListed))
			break
		}
		parts = append(parts, fmt.Sprintf("%s (%s, %s)", rel(p.Project, c.Path), humanBytes(c.Size), c.Lifespan))
	}
	return strings.Join(parts, ", ")
}

// describeProposal is a factual summary for the agent. Hook text is written
// as statements, not commands, as the Claude Code docs advise.
func describeProposal(p core.Proposal, consent string) string {
	text := describeProposalCandidates(p, consent, listCandidates(p))
	if len(text) > maxContext/2-1 {
		text = describeProposalCandidates(p, consent, "the complete candidate list is available through `obit proposals`")
	}
	return text
}

func describeProposalCandidates(p core.Proposal, consent, candidates string) string {
	switch p.Kind {
	case core.ProposalContainer:
		return fmt.Sprintf("Obituary proposal #%d: containers created during a session are awaiting the user's decision: %s. `obit approve %d` assigns lifespans; `obit reject %d` drops this proposal. Expiry stops containers and preserves their writable layers.", p.ID, candidates, p.ID, p.ID)
	case core.ProposalOnboard:
		if consent == core.ConsentApproved {
			return fmt.Sprintf("Obituary proposal #%d: automatic registration in this project is already approved. These candidates remain pending after temporary problems: %s. `obit approve %d` is available to retry them.",
				p.ID, candidates, p.ID)
		}
		return fmt.Sprintf("Obituary (讣告) proposal #%d: %s in this project could get lifespans by rule, %s in total: %s. "+
			"Obituary registers and deletes nothing here until the user approves; approval also lets it handle such artefacts here automatically from then on. "+
			"The user can approve with `obit approve %d` or decline with `obit reject %d`.",
			p.ID, textutil.Count(len(p.Candidates), "agent artefact"), humanBytes(p.Size()), candidates, p.ID, p.ID)
	case core.ProposalLarge:
		return fmt.Sprintf("Obituary proposal #%d: %s (%s) matched a rule; awaiting approval: %s. `obit approve %d` registers the listed candidates.",
			p.ID, textutil.Count(len(p.Candidates), "unusually large artefact"), humanBytes(p.Size()), candidates, p.ID)
	default:
		return fmt.Sprintf("Obituary proposal #%d: an earlier session left %s that no rule covers (%s): %s. "+
			"No lifespan is assigned and nothing happens unless approved: `obit approve %d` gives each %s, `obit mark <path> <lifespan>` handles one, `obit reject %d` drops the list.",
			p.ID, textutil.Count(len(p.Candidates), "untracked item"), humanBytes(p.Size()), candidates, p.ID, auto.LeftoverLifespan, p.ID)
	}
}

// Only acknowledge complete summaries that fit the actual hook output budget.
func appendContext(lines []string, text string) ([]string, bool) {
	size := len(text)
	for _, line := range lines {
		size += len(line) + 1
	}
	if size > maxContext {
		return lines, false
	}
	return append(lines, text), true
}

// sessionContext tells the agent what needs the user: paused project rules
// and pending proposals. OBIT.md already says Obituary is present, and grave
// notices announce buried paths when a tool call touches them, so the list of
// expiring items and recent burials is opt-in ([hooks] session_summary) to
// save the agent's tokens. Empty when there is nothing worth saying.
func sessionContext(e *auto.Engine, root, session string) (delivery, error) {
	var due []core.Item
	if e.Svc.SessionSummary {
		var err error
		if due, err = e.Svc.DueWithin(root, 24*time.Hour); err != nil {
			return delivery{}, err
		}
	}
	state, err := e.Svc.Store.Read()
	if err != nil {
		return delivery{}, err
	}
	var lines []string
	if _, rulesState, err := e.RuleSet(root); err != nil {
		Logf(e.Svc.Store.Home, "project rules paused: %v", err)
		lines, _ = appendContext(lines, rules.PauseDiagnostic(root, rulesState, err))
	}
	if len(due) > 0 {
		var parts []string
		for i, it := range due {
			if i == maxListed {
				parts = append(parts, fmt.Sprintf("and %d more", len(due)-maxListed))
				break
			}
			parts = append(parts, fmt.Sprintf("#%d %s (%s)", it.ID, rel(root, it.Path), textutil.DueIn(time.Until(it.ExpiresAt))))
		}
		destination := "buried (restorable with `obit resurrect`)"
		if e.Svc.Mode == core.ModeRecycle {
			destination = "sent to the system Recycle Bin (restore from the Recycle Bin)"
		}
		text := fmt.Sprintf("Obituary (讣告): %s in this project will expire within 24 hours and will be %s: %s. `obit extend <id> <lifespan>` keeps one longer.",
			textutil.Count(len(due), "item"), destination, strings.Join(parts, ", "))
		if len(text) > maxContext/2 {
			text = fmt.Sprintf("Obituary: %s will expire within 24 hours; `obit list` shows the list and `obit extend <id> <lifespan>` keeps one longer.", textutil.Count(len(due), "item"))
		}
		lines = append(lines, text)
	}
	if text := graveSessionContext(state.Burials, root, session); e.Svc.SessionSummary && e.Svc.Mode != core.ModeRecycle && text != "" {
		lines, _ = appendContext(lines, text)
	}
	e.Svc.Store.RefreshGraveNoticeIndex()
	context := delivery{lines: lines}
	// Resume/compaction refreshes old notices too, after unseen proposals have
	// had a chance to fit. Otherwise a truncated tail could starve forever.
	slices.SortStableFunc(state.Proposals, func(a, b core.Proposal) int {
		aTold, bTold := slices.Contains(a.Notified, session), slices.Contains(b.Notified, session)
		if aTold == bTold {
			return 0
		}
		if aTold {
			return 1
		}
		return -1
	})
	for _, p := range state.Proposals {
		if strings.EqualFold(p.Project, root) || p.Kind == core.ProposalContainer && p.Session == session {
			visible := core.ExistingProposals([]core.Proposal{p})
			if len(visible) == 0 {
				continue
			}
			p = visible[0]
			context.append(p, state.Projects[core.ProjectKey(p.Project)].Consent)
		}
	}
	return context, nil
}
