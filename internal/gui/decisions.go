package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/rules"
	"github.com/woodfishhhh/obituary/internal/watch"
)

type ReviewedProposal struct {
	core.Proposal
	Fingerprint string `json:"fingerprint"`
	Blocked     string `json:"blocked,omitempty"`
}

type ProposalList struct {
	Proposals []ReviewedProposal `json:"proposals"`
	// Handled is what the proposal mode let in without asking, while tracked.
	Handled []core.AutoHandled `json:"handled"`
	// Mode is [proposals] mode: tiered, auto or ask.
	Mode string `json:"mode"`
}
type ProposalReply struct {
	Results []core.ProposalResult `json:"results"`
	Error   string                `json:"error,omitempty"`
}

func (a *API) Proposals() (ProposalList, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	state, err := a.deps.Service.Store.Inspect()
	rows := ProposalList{Proposals: []ReviewedProposal{}}
	// The fingerprint covers exactly the displayed candidates. A partial view
	// cannot authorize the full ledger snapshot; background pruning refreshes it.
	for _, p := range app.ProposalViews(a.deps.Service, state.Proposals) {
		rows.Proposals = append(rows.Proposals, ReviewedProposal{Proposal: p.Proposal, Fingerprint: core.ProposalFingerprint(p.Proposal), Blocked: p.Blocked})
	}
	rows.Handled = state.AutoHandled()
	cfg, cfgErr := a.readConfig()
	rows.Mode = cfg.Proposals.Mode
	if cfgErr != nil {
		rows.Mode = a.deps.Service.Proposals()
	}
	return rows, err
}

// PlanProposalMode previews [proposals] mode from the Decisions page. Hooks
// read the config on every event; the service here follows at once.
func (a *API) PlanProposalMode(mode string) (Plan, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	path := filepath.Join(a.deps.ConfigDir, "config.toml")
	old, err := a.readConfig()
	if err != nil {
		return Plan{}, err
	}
	if old.Proposals.Mode == mode {
		return a.savePlan("proposalMode", []ChangePreview{}, func() error { return nil }), nil
	}
	change, err := integrate.PlanProposalModeConfig(path, mode)
	if err != nil {
		return Plan{}, err
	}
	preview := []ChangePreview{{Target: path, Before: "[proposals] mode=" + old.Proposals.Mode, After: "[proposals] mode=" + mode}}
	return a.savePlan("proposalMode", preview, func() error {
		if err := integrate.Apply([]integrate.Change{change}); err != nil {
			return err
		}
		cfg, err := a.readConfig()
		if err == nil {
			a.deps.Service.ProposalMode = cfg.Proposals.Mode
		}
		return err
	}), nil
}

func (a *API) Approve(refs, fingerprints []string) (ProposalReply, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := requireRefs(refs); err != nil {
		return ProposalReply{Results: []core.ProposalResult{}}, err
	}
	// Request-local rules also observe edits made outside this GUI.
	engine := &auto.Engine{Svc: a.deps.Service, Trust: a.deps.Engine.Trust}
	results, err := engine.ApproveReviewed(refs, fingerprints)
	// Watch approvals are handled by Engine.Approve, as in the CLI. Its
	// registrations are reconciled by the resident's directory notifications.
	return ProposalReply{Results: list(results), Error: errorText(err)}, nil
}

func (a *API) Reject(refs, fingerprints []string) (ProposalReply, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := requireRefs(refs); err != nil {
		return ProposalReply{Results: []core.ProposalResult{}}, err
	}
	results, err := a.deps.Service.RejectReviewed(refs, fingerprints)
	return ProposalReply{Results: list(results), Error: errorText(err)}, nil
}

type RuleFile struct {
	rules.FileState
	Content string `json:"content"`
}
type RuleList struct {
	Rules []RuleFile `json:"rules"`
}

// Rules lists files in projects known to this ledger, including trusted files
// so the user can revoke trust. It never recursively scans unrelated folders.
func (a *API) Rules() (RuleList, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ruleFiles()
}

func (a *API) ruleFiles() (RuleList, error) {
	state, err := a.deps.Service.Store.Inspect()
	if err != nil {
		return RuleList{}, err
	}
	rows := RuleList{Rules: []RuleFile{}}
	seen := map[string]bool{}
	var projects []os.FileInfo
	for _, root := range projectRoots(state) {
		file, err := a.deps.Engine.Trust.Check(filepath.Join(root, ".obitrules"))
		if err != nil {
			return rows, err
		}
		if file.Status == rules.Absent || seen[file.Path] {
			continue
		}
		// Case aliases share a directory identity even when canonical path
		// spelling is preserved. Distinct case-sensitive projects stay separate.
		if info, err := os.Stat(root); err == nil {
			if slices.ContainsFunc(projects, func(known os.FileInfo) bool { return os.SameFile(info, known) }) {
				continue
			}
			projects = append(projects, info)
		} else if !os.IsNotExist(err) {
			return rows, err
		}
		rows.Rules = append(rows.Rules, RuleFile{FileState: file, Content: file.Content()})
		seen[file.Path] = true
	}
	return rows, nil
}

func projectRoots(state core.State) []string {
	roots := []string{}
	for root := range state.Projects {
		roots = append(roots, root)
	}
	for _, p := range state.Proposals {
		roots = append(roots, p.Project)
	}
	for _, i := range state.Items {
		roots = append(roots, i.Project)
	}
	for _, b := range state.Burials {
		roots = append(roots, b.Project)
	}
	for _, w := range state.Watches {
		roots = append(roots, w.Project)
	}
	slices.Sort(roots)
	return slices.DeleteFunc(slices.Compact(roots), func(root string) bool { return root == "" })
}

// Trust requires the digest the user reviewed, just like obit trust. Edits
// made while a confirmation is open cannot silently become trusted.
func (a *API) Trust(path, reviewedSHA256 string) (RuleFile, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if reviewedSHA256 == "" {
		return RuleFile{}, fmt.Errorf("review the rules and provide their sha256 before trusting")
	}
	file, err := a.deps.Engine.Trust.Trust(path, reviewedSHA256)
	return RuleFile{FileState: file, Content: file.Content()}, err
}

func (a *API) Untrust(path string) (Receipt, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	changed, err := a.deps.Engine.Trust.Untrust(path)
	return Receipt{Action: "untrust", Path: path, Changed: changed}, err
}

type WatchList struct {
	Policies []core.WatchPolicy `json:"policies"`
}
type WatchReply struct {
	Policy core.WatchPolicy `json:"policy"`
	Error  string           `json:"error,omitempty"`
}

func (a *API) Watches() (WatchList, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	policies, err := a.deps.Service.Watches()
	return WatchList{Policies: list(policies)}, err
}

func (a *API) SetWatch(path, lifespan, age string) (WatchReply, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if age == "" {
		age = core.WatchFirstSeen
	}
	policy, err := a.deps.Service.SetWatch(path, lifespan, age)
	if err != nil {
		return WatchReply{Policy: policy, Error: errorText(err)}, nil
	}
	err = watch.Reconcile(a.deps.Service)
	return WatchReply{Policy: policy, Error: errorText(err)}, nil
}

func (a *API) RemoveWatch(path string) (Receipt, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	err := a.deps.Service.RemoveWatch(path)
	return Receipt{Action: "removeWatch", Path: path, Changed: err == nil}, err
}
