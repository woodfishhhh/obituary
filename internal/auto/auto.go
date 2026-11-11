// Package auto registers the artefacts agents leave behind, by rule, without
// anyone typing `obit add`. It is deliberately conservative: a path is only
// registered if it lies in a Git work tree, a rule matches it, Git tracks
// neither it nor anything inside it, it has no lifespan yet and the guard
// accepts it. Anything uncertain is skipped, never registered.
package auto

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"github.com/woodfishhhh/obituary/internal/project"
	"github.com/woodfishhhh/obituary/internal/rules"
)

// Engine applies rules on top of a service.
type Engine struct {
	Svc     *core.Service
	Trust   rules.TrustStore
	sets    map[string]cachedRules
	tracked func(project.Project, string) (bool, error) // optional query seam for tests
}

type cachedRules struct {
	set   rules.Set
	state rules.FileState
}

// Options describe where registrations come from.
type Options struct {
	Source   string // defaults to "rule"
	Session  string
	DryRun   bool
	Observed *core.Observation // hook's original object, checked at ledger commit
}

// Decision is what happened to one path.
type Decision struct {
	Path      string     `json:"path"`
	Unit      string     `json:"unit,omitempty"`
	Lifespan  string     `json:"lifespan,omitempty"`
	Rule      string     `json:"rule,omitempty"`
	Action    string     `json:"action"` // registered, would-register, proposed, skipped, retry
	Reason    string     `json:"reason,omitempty"`
	Warning   string     `json:"warning,omitempty"`
	Err       error      `json:"-"` // retryable failure, preserved for queue acknowledgement
	Item      *core.Item `json:"item,omitempty"`
	rulesFile string
}

// Decision actions.
const (
	Registered    = "registered"
	WouldRegister = "would-register"
	Skipped       = "skipped"
	Retry         = "retry"
	Proposed      = "proposed"
)

func failedDecision(d Decision, err error) Decision {
	d.Action, d.Reason, d.Err = Retry, err.Error(), err
	return d
}

// Only an inspection of this candidate can definitively refuse it. A failed
// lookup of some other tracked object remains retryable, even for ErrUnsafe.
func inspectionDecision(d Decision, err error) Decision {
	d = failedDecision(d, err)
	var refused *core.RefusedError
	if errors.As(err, &refused) || errors.Is(err, core.ErrUnsafe) || errors.Is(err, core.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		d.Action, d.Err = Skipped, nil
	}
	return d
}

const reasonNoRule = "no rule gives it a lifespan"

// RuleSet returns the rules in force for a project root.
func (e *Engine) RuleSet(root string) (rules.Set, rules.FileState, error) {
	root, err := pathutil.LongAbs(root)
	if err != nil {
		return rules.Set{}, rules.FileState{}, err
	}
	if set, ok := e.sets[root]; ok {
		return set.set, set.state, nil
	}
	set, state, err := rules.Load(root, e.Trust)
	if err == nil {
		if e.sets == nil {
			e.sets = map[string]cachedRules{}
		}
		e.sets[root] = cachedRules{set: set, state: state}
	}
	return set, state, err
}

// Apply registers, by rule, the units the given paths belong to.
func (e *Engine) Apply(paths []string, opts Options) ([]Decision, error) {
	e.sets = nil
	return e.apply(paths, opts, locations{})
}

func (e *Engine) apply(paths []string, opts Options, located locations) ([]Decision, error) {
	source := opts.Source
	if source == "" {
		source = "rule"
	}
	state, err := e.Svc.Store.Read()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	warned := map[string]bool{}
	var out []Decision
	for _, ref := range paths {
		d := e.decide(ref, &state, seen, located)
		if d.Warning != "" {
			if warned[d.rulesFile] {
				d.Warning = ""
			}
			warned[d.rulesFile] = true
		}
		if d.Action != WouldRegister {
			out = append(out, d)
			continue
		}
		if opts.DryRun {
			out = append(out, d)
			continue
		}
		lifespan, _ := core.ParseDuration(d.Lifespan)
		proj, ok, err := project.Find(d.Unit)
		if err != nil || !ok {
			d.Action, d.Reason = Skipped, "could not confirm its Git work tree"
			if err != nil {
				d = failedDecision(d, err)
			}
			out = append(out, d)
			continue
		}
		results, err := e.Svc.Add([]string{d.Unit}, core.AddOptions{
			ExpiresAt: e.now().Add(lifespan), Source: source, Project: proj.Root,
			Session: opts.Session, Note: "rule " + d.Rule, OnlyNew: true,
			Observed: opts.Observed,
		})
		if err != nil && results == nil {
			return out, err
		}
		if results[0].Error != "" {
			cause := results[0].Err
			if cause == nil {
				cause = errors.New(results[0].Error)
			}
			d = inspectionDecision(d, cause)
		} else {
			d.Action, d.Item = Registered, results[0].Item
			state.Items = append(state.Items, *results[0].Item)
		}
		out = append(out, d)
	}
	return out, nil
}

func (e *Engine) now() time.Time {
	if e.Svc.Now != nil {
		return e.Svc.Now()
	}
	return time.Now()
}

func (e *Engine) decide(ref string, state *core.State, seen map[string]bool, located locations) Decision {
	d, proj := e.matchRule(ref, seen)
	return e.vetDecision(d, proj, state, located)
}

func (e *Engine) vetDecision(d Decision, proj project.Project, state *core.State, located locations) Decision {
	if d.Action != WouldRegister {
		return d
	}
	if reason, err := e.vetUnit(proj, d.Unit, state, located); err != nil {
		return failedDecision(d, err)
	} else if reason != "" {
		d.Action, d.Reason = Skipped, reason
	}
	return d
}

// matchRule only selects a possible unit. It does not authorize registration
// or a proposal: both still pass vetUnit against live Git and lifespan state.
func (e *Engine) matchRule(ref string, seen map[string]bool) (Decision, project.Project) {
	var proj project.Project
	skip := func(d Decision, reason string) Decision {
		d.Action, d.Reason = Skipped, reason
		return d
	}
	d := Decision{Path: ref}
	abs, _, err := e.Svc.FS.LongPath(ref)
	if err != nil {
		return failedDecision(d, err), proj
	}
	d.Path = abs
	info, err := os.Lstat(abs)
	if err != nil {
		return inspectionDecision(d, err), proj
	}
	// Decide on the real location: a junction inside one project must not
	// make a path in another project look like it belongs here.
	real, err := e.Svc.FS.RealPath(abs)
	if err != nil {
		return inspectionDecision(d, fmt.Errorf("could not resolve its real location: %w", err)), proj
	}
	proj, ok, err := project.Find(real)
	if err != nil {
		return failedDecision(d, fmt.Errorf("could not find its Git work tree: %w", err)), proj
	}
	if !ok {
		return skip(d, "not inside a Git work tree"), proj
	}
	set, file, err := e.RuleSet(proj.Root)
	d.Warning, d.rulesFile = file.Warning, file.Path
	if err != nil {
		return failedDecision(d, fmt.Errorf("rules: %w", err)), proj
	}
	m, ok, err := set.Match(real, info.IsDir())
	if err != nil {
		return failedDecision(d, fmt.Errorf("rules: %w", err)), proj
	}
	if !ok {
		d.Unit = real
		return skip(d, reasonNoRule), proj
	}
	d.Unit, d.Lifespan, d.Rule = m.Unit, formatLifespan(m.Lifespan), fmt.Sprintf("%s (%s:%d)", m.Rule.Pattern, filepath.Base(m.Rule.Source), m.Rule.Line)
	if seen[strings.ToLower(m.Unit)] {
		return skip(d, "already handled in this batch"), proj
	}
	seen[strings.ToLower(m.Unit)] = true
	d.Action = WouldRegister
	return d, proj
}

// vetUnit is the check every automatic registration passes, whether it
// comes from a rule, a session's leftovers or an approved proposal: Git must
// not track the unit or anything in it, and it must not shorten an existing
// lifespan, be it the same object under another name, an item inside it or
// an item it lies in. A reason is a definite refusal; an error is retryable.
func (e *Engine) vetUnit(proj project.Project, unit string, state *core.State, located locations) (string, error) {
	id, err := e.Svc.FS.Identify(unit)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) || errors.Is(err, core.ErrUnsafe) {
			return "cannot identify it: " + err.Error(), nil
		}
		return "", fmt.Errorf("cannot identify it: %w", err)
	}
	if idx := state.FindIdentity(id); idx >= 0 {
		return fmt.Sprintf("already has a lifespan (#%d)", state.Items[idx].ID), nil
	}
	if exempt, err := e.Svc.AutomaticExempt(state, unit, id); err != nil {
		return "", err
	} else if exempt {
		return "kept by resurrect or spare until its next write", nil
	}
	scope := e.Svc.RegistrationScope(unit, id)
	defer scope.Close()
	for _, it := range state.Items {
		if !scope.Related(it) {
			continue
		}
		if err := scope.CheckIdentity(it); err != nil {
			return "", err
		}
		path := it.Path
		cached := false
		if it.Identity != nil && (scope != nil || it.Identity.Serial == id.Serial) {
			_, cached = located[*it.Identity]
			if current, err := located.locate(e.Svc.FS, *it.Identity); err == nil {
				path = current
			} else if errors.Is(err, core.ErrNotFound) {
				continue
			} else {
				return "", fmt.Errorf("could not locate #%d: %w", it.ID, err)
			}
		}
		if cached && lifespanConflict(path, unit, it.ID) != "" {
			// Cached positions may only suggest a refusal. Confirm it against
			// the live object before settling a proposal or skipping a unit.
			delete(located, *it.Identity)
			current, err := located.locate(e.Svc.FS, *it.Identity)
			if err == nil {
				path = current
			} else if errors.Is(err, core.ErrNotFound) {
				continue
			} else {
				return "", fmt.Errorf("could not locate #%d: %w", it.ID, err)
			}
		}
		if reason := lifespanConflict(path, unit, it.ID); reason != "" {
			return reason, nil
		}
	}
	tracked, err := e.trackedByGit(proj, unit)
	switch {
	case err != nil:
		return "", fmt.Errorf("could not ask Git whether it is tracked: %w", err)
	case tracked:
		return "Git tracks it (or something inside it)", nil
	}
	return "", nil
}

func (e *Engine) trackedByGit(proj project.Project, path string) (bool, error) {
	if e.tracked != nil {
		return e.tracked(proj, path)
	}
	return proj.Tracked(path)
}

func lifespanConflict(path, unit string, id int64) string {
	switch {
	case strings.EqualFold(filepath.Clean(path), filepath.Clean(unit)):
		return fmt.Sprintf("already has a lifespan (#%d)", id)
	case inside(path, unit):
		return fmt.Sprintf("holds #%d, which has its own lifespan", id)
	case inside(unit, path):
		return fmt.Sprintf("lies inside #%d, which already has a lifespan", id)
	}
	return ""
}

type location struct {
	path string
	err  error
}

// locations belongs to one batch, including nested Observe/Apply calls.
// Cache errors too, without changing their retry/not-found semantics.
type locations map[core.Identity]location

func (l locations) locate(fs core.FS, id core.Identity) (string, error) {
	got, ok := l[id]
	if !ok {
		got.path, got.err = fs.Locate(id)
		l[id] = got
	}
	return got.path, got.err
}

// inside reports whether child lies strictly below parent.
func inside(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)
}

// RecheckRules is core.Service.Recheck: before an automatic item is buried,
// the project's rules must not protect it or anything inside it.
func (e *Engine) RecheckRules(root, path string, isDir bool) error {
	e.sets = nil // rules may have changed since this engine cached them
	set, _, err := e.RuleSet(root)
	if err != nil {
		return fmt.Errorf("could not load the project's rules: %w", err)
	}
	protected, err := set.Protects(path, isDir)
	if err != nil {
		return fmt.Errorf("could not check protection rules: %w", err)
	}
	if protected {
		return core.ErrProtected
	}
	return nil
}
