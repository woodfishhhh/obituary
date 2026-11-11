package auto

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"github.com/woodfishhhh/obituary/internal/project"
)

// Limits above which a single automatic registration needs approval.
var (
	LargeBytes int64 = 1 << 30
	LargeFiles       = 200
)

// LeftoverLifespan is what approved leftovers get.
const LeftoverLifespan = "3d"

// Notice tells the agent about a proposal it has not heard of yet.
type Notice struct {
	Proposal core.Proposal
	Created  bool
}

// size returns the bytes and regular files under path, stopping early once
// either limit is exceeded (the result is then only a lower bound): a hook
// only needs to know "large", and must not walk a huge tree to learn it.
func size(path string) (int64, int) {
	var bytes int64
	files := 0
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type().IsRegular() {
			files++
			if info, err := d.Info(); err == nil {
				bytes += info.Size()
			}
		}
		if bytes > LargeBytes || files > LargeFiles {
			return fs.SkipAll
		}
		return nil
	})
	return bytes, files
}

// Observe handles paths an agent just wrote. In a project whose owner
// approved automatic registration they are registered by rule (unusually
// large units become a proposal instead); in a project never asked, the
// would-be registrations become an onboard proposal; in a declined project
// nothing happens. It returns the proposals the session should hear about.
func (e *Engine) Observe(paths []string, session string) ([]Decision, []Notice, error) {
	e.sets = nil
	// An unidentifiable write cannot release a keep decision; the ordinary
	// per-path inspection below still reports that path's failure or refusal.
	rows, _ := e.Svc.CaptureObservations(paths, session)
	for _, row := range rows {
		if err := e.Svc.ObserveAutomaticWrite(row); err != nil {
			return nil, nil, err
		}
	}
	return e.observe(paths, session, locations{}, nil)
}

// ObserveCaptured preserves each hook event's identity across lock waits.
// Decisions keep the input order, so retries queue the exact captured event.
func (e *Engine) ObserveCaptured(rows []core.Observation) ([]Decision, []Notice, error) {
	e.sets = nil
	var decisions []Decision
	var notices []Notice
	var failures error
	located := locations{}
	for _, row := range rows {
		if err := e.Svc.ObserveAutomaticWrite(row); err != nil {
			decisions = append(decisions, inspectionDecision(Decision{Path: row.Path}, err))
			failures = errors.Join(failures, err)
			continue
		}
		ds, ns, err := e.observe([]string{row.Path}, row.Session, located, &row)
		decisions = append(decisions, ds...)
		failures = errors.Join(failures, err)
		for _, n := range ns {
			idx := slices.IndexFunc(notices, func(old Notice) bool { return old.Proposal.ID == n.Proposal.ID })
			if idx < 0 {
				notices = append(notices, n)
			} else {
				notices[idx] = n
			}
		}
	}
	return decisions, notices, failures
}

// observe aborts only when the initial ledger read fails. Individual inspection,
// registration and proposal failures stay in their decisions and are joined
// after every input has been processed.
func (e *Engine) observe(paths []string, session string, located locations, observed *core.Observation) ([]Decision, []Notice, error) {
	opts := Options{Session: core.NormalizeSession(session), Observed: observed}
	state, err := e.Svc.Store.Read()
	if err != nil {
		var decisions []Decision
		for _, path := range paths {
			decisions = append(decisions, failedDecision(Decision{Path: path}, err))
		}
		return decisions, nil, err
	}
	var decisions []Decision
	var notices []Notice
	var failures error
	for _, ref := range paths {
		d, n := e.observePath(ref, opts, &state, located)
		if n != nil {
			notices = mergeObserveNotice(notices, *n)
		}
		decisions = append(decisions, d)
		failures = errors.Join(failures, d.Err)
	}
	return decisions, notices, failures
}

func (e *Engine) observePath(ref string, opts Options, state *core.State, located locations) (Decision, *Notice) {
	if state.ObservationCompleted(opts.Observed) {
		return Decision{Path: ref, Action: Skipped, Reason: "observation already completed"}, nil
	}
	d, matchedProject := e.matchRule(ref, map[string]bool{})
	// Approved units are vetted by Apply after sizing. Asking Git here
	// too used to launch two identical ls-files processes per hook. Keep
	// fresh queries instead of caching index answers across either writes
	// or hook calls (worktrees and index replacement must stay Git's job).
	if state.Projects[core.ProjectKey(matchedProject.Root)].Consent != core.ConsentApproved {
		d = e.vetDecision(d, matchedProject, state, located)
	}
	var n *Notice
	if d.Action == WouldRegister {
		d, n = e.observeRegistration(ref, d, matchedProject, opts, state, located)
	}
	if opts.Observed != nil && d.Action == Skipped {
		// A missing path during inspection is not definitive if its volume
		// disconnected after the replay's initial identity check.
		if err := e.Svc.CheckObservation(opts.Observed); err != nil {
			d = inspectionDecision(d, err)
		}
	}
	return d, n
}

func (e *Engine) observeRegistration(ref string, d Decision, matchedProject project.Project, opts Options, state *core.State, located locations) (Decision, *Notice) {
	proj, ok, err := project.Find(d.Unit)
	if err != nil {
		return failedDecision(d, err), nil
	}
	if !ok {
		d.Action, d.Reason = Skipped, "not inside a Git work tree"
		return d, nil
	}
	if state.Projects[core.ProjectKey(proj.Root)].Consent == core.ConsentDeclined {
		d.Action, d.Reason = Skipped, "automatic registration declined"
		return d, nil
	}
	bytes, files := size(d.Unit)
	approved := state.Projects[core.ProjectKey(proj.Root)].Consent == core.ConsentApproved
	if approved && bytes <= LargeBytes && files <= LargeFiles {
		return e.applyObservedUnit(ref, d, opts, state, located), nil
	}
	// Approved large units bypass Apply, so they need its vetting
	// gate before becoming proposals as well.
	if state.Projects[core.ProjectKey(matchedProject.Root)].Consent == core.ConsentApproved {
		d = e.vetDecision(d, proj, state, located)
	}
	if d.Action != WouldRegister {
		return d, nil
	}
	return e.proposeObservedUnit(d, proj.Root, opts, bytes, approved)
}

func (e *Engine) applyObservedUnit(ref string, d Decision, opts Options, state *core.State, located locations) Decision {
	applied, err := e.apply([]string{d.Unit}, opts, located)
	if len(applied) == 0 {
		return failedDecision(d, err)
	}
	d = applied[0]
	d.Path = ref
	if d.Item != nil {
		state.Items = append(state.Items, *d.Item)
	}
	return d
}

func (e *Engine) proposeObservedUnit(d Decision, root string, opts Options, bytes int64, approved bool) (Decision, *Notice) {
	kind := core.ProposalOnboard
	if approved {
		kind = core.ProposalLarge
	}
	n, err := e.propose(kind, root, opts.Session, []core.Candidate{{Path: d.Unit, Lifespan: d.Lifespan, Rule: d.Rule, Size: bytes}}, opts.Observed)
	if err != nil {
		return inspectionDecision(d, err), nil
	}
	d.Action = Proposed
	return d, n
}

func mergeObserveNotice(notices []Notice, n Notice) []Notice {
	idx := slices.IndexFunc(notices, func(old Notice) bool { return old.Proposal.ID == n.Proposal.ID })
	if idx < 0 {
		return append(notices, n)
	}
	notices[idx].Proposal = n.Proposal
	return notices
}

func (e *Engine) propose(kind, root, session string, cands []core.Candidate, observed *core.Observation) (*Notice, error) {
	if len(cands) == 0 {
		return nil, nil
	}
	p, created, added, err := e.Svc.ProposeObserved(kind, root, session, cands, observed)
	if err != nil || p.ID == 0 || (!created && added == 0 && slices.Contains(p.Notified, session)) {
		return nil, err
	}
	return &Notice{Proposal: p, Created: created}, nil
}

// Session snapshots let a session's leftovers be found: what was untracked
// when it started is not its doing. They live in the data directory, so a
// repository is not touched just because an agent opened it.

type snapshot struct {
	Project string    `json:"project"`
	Session string    `json:"session"`
	At      time.Time `json:"at"`
	Paths   []string  `json:"paths"`
}

var unsafeSession = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

func (e *Engine) snapshotPath(root, session string) string {
	sum := sha256.Sum256([]byte(core.ProjectKey(root)))
	return filepath.Join(e.Svc.Store.Home, "sessions", hex.EncodeToString(sum[:6])+"-"+unsafeSession.ReplaceAllString(session, "_")+".json")
}

// Snapshot records the project's untracked paths at the start of a session.
// A valid snapshot for the session is kept (a resumed session); an unreadable
// one is replaced. It is published atomically, so a killed hook never leaves
// a half-written file behind.
func (e *Engine) Snapshot(root, session string) error {
	root, err := pathutil.LongAbs(root)
	if err != nil {
		return err
	}
	path := e.snapshotPath(root, session)
	if _, err := e.readSnapshot(path); err == nil {
		return nil
	}
	paths, err := project.Project{Root: root}.Untracked()
	if err != nil {
		return err
	}
	return e.saveSnapshot(path, snapshot{Project: root, Session: session, At: e.now(), Paths: paths})
}

func (e *Engine) saveSnapshot(path string, snap snapshot) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	e.pruneSnapshots(filepath.Dir(path))
	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "snapshot-*.tmp")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

func (e *Engine) readSnapshot(path string) (snapshot, error) {
	var snap snapshot
	data, err := os.ReadFile(path)
	if err != nil {
		return snap, err
	}
	if err := json.Unmarshal(data, &snap); err != nil {
		return snap, err
	}
	return snap, nil
}

func (e *Engine) pruneSnapshots(dir string) {
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if info, err := entry.Info(); err == nil && e.now().Sub(info.ModTime()) > 30*24*time.Hour {
			_ = os.Remove(filepath.Join(dir, entry.Name())) // our own snapshot file
		}
	}
}

// Leftovers compares the project with the session's snapshot. New paths a
// rule covers go through Observe; the rest, which no rule covers, become a
// leftovers proposal: they are listed for the agent and the user to decide,
// never registered on their own, because a new untracked file may be fresh
// work that simply has not been committed yet.
func (e *Engine) Leftovers(root, session string) ([]Notice, error) {
	e.sets = nil
	root, err := pathutil.LongAbs(root)
	if err != nil {
		return nil, err
	}
	snap, err := e.readSnapshot(e.snapshotPath(root, session))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	before := map[string]bool{}
	for _, p := range snap.Paths {
		before[strings.ToLower(filepath.Clean(p))] = true
	}
	// A path predates the session if it or any directory above it was in the
	// snapshot: Git collapses a wholly untracked directory into one entry,
	// and expands it once something inside is tracked.
	predates := func(p string) bool {
		for dir := filepath.Clean(p); ; {
			if before[strings.ToLower(dir)] {
				return true
			}
			parent := filepath.Dir(dir)
			if parent == dir || len(parent) < len(root) {
				return false
			}
			dir = parent
		}
	}
	now, err := project.Project{Root: root}.Untracked()
	if err != nil {
		return nil, err
	}
	work := strings.ToLower(project.Project{Root: root}.WorkDir())
	state, err := e.Svc.Store.Read()
	if err != nil {
		return nil, err
	}
	var fresh []string
	for _, p := range now {
		lp := strings.ToLower(p)
		if predates(p) || state.LeftoverRejected(root, session, p) || lp == work || strings.HasPrefix(lp, work+string(filepath.Separator)) {
			continue
		}
		fresh = append(fresh, p)
	}
	if len(fresh) == 0 {
		return nil, nil
	}
	located := locations{}
	_, notices, observeErr := e.observe(fresh, session, located, nil)
	state, err = e.Svc.Store.Read()
	if err != nil {
		return notices, err
	}
	var cands []core.Candidate
	seen := map[string]bool{}
	for _, p := range fresh {
		d := e.decide(p, &state, seen, located)
		if d.Action != Skipped || d.Reason != reasonNoRule {
			continue // handled by rule, already tracked, or not ours to touch
		}
		proj, ok, err := project.Find(d.Unit)
		if err != nil || !ok {
			continue
		}
		if reason, err := e.vetUnit(proj, d.Unit, &state, located); err != nil || reason != "" {
			continue
		}
		set, _, err := e.RuleSet(proj.Root)
		if err != nil {
			continue
		}
		if protected, err := set.Protects(d.Unit, isDirectory(d.Unit)); err != nil || protected {
			continue
		}
		bytes, _ := size(d.Unit)
		cands = append(cands, core.Candidate{Path: d.Unit, Lifespan: LeftoverLifespan, Size: bytes})
	}
	n, err := e.propose(core.ProposalLeftovers, root, session, cands, nil)
	if n != nil {
		notices = append(notices, *n)
	}
	return notices, errors.Join(observeErr, err)
}

func isDirectory(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.IsDir()
}
