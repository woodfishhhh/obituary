package auto

import (
	"errors"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/docker"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"github.com/woodfishhhh/obituary/internal/project"
)

// SnapshotSession records only time for sessions outside a Git project.
func (e *Engine) SnapshotSession(root, session string) error {
	root, err := pathutil.LongAbs(root)
	if err != nil {
		return err
	}
	path := e.snapshotPath(root, session)
	if _, err := e.readSnapshot(path); err == nil {
		return nil
	}
	return e.saveSnapshot(path, snapshot{Project: root, Session: session, At: e.now()})
}

// ContainerLeftovers runs only in the background finish process. Snapshot writes
// only its existing timestamp; hook entry points never enumerate Docker.
func (e *Engine) ContainerLeftovers(root, session string) ([]Notice, error) {
	if e.Svc.Docker == nil || e.off() {
		return nil, nil
	}
	root, err := pathutil.LongAbs(root)
	if err != nil {
		return nil, err
	}
	snap, err := e.readSnapshot(e.snapshotPath(root, session))
	if err != nil || snap.At.IsZero() {
		return nil, nil
	}
	end := e.now()
	rows, err := e.Svc.Docker.List(e.Svc.Store.Context())
	if err != nil {
		return nil, nil
	} // optional daemon unavailable: quietly retry next finish
	state, err := e.Svc.Store.Read()
	if err != nil {
		return nil, err
	}
	groups := map[string][]core.Candidate{}
	for _, row := range rows {
		if row.Created.Before(snap.At) || row.Created.After(end) || knownContainer(state, row.Identity) {
			continue
		}
		owner, reason, err := projectContainer(row)
		if err != nil {
			return nil, err
		}
		if owner == "" {
			continue
		}
		id := row.Identity
		groups[owner] = append(groups[owner], core.Candidate{Kind: core.KindContainer, Container: &id, Path: id.Display(), Lifespan: LeftoverLifespan, Reason: reason})
	}
	var notices []Notice
	for owner, candidates := range groups {
		notice, err := e.propose(core.ProposalContainer, owner, session, candidates, nil)
		if notice != nil {
			notice, err = e.settleNotice(*notice, err)
		}
		if err != nil {
			return notices, err
		}
		if notice != nil {
			notices = append(notices, *notice)
		}
	}
	return notices, nil
}

func knownContainer(state core.State, id docker.Identity) bool {
	for _, it := range state.Items {
		if it.Container != nil && it.Container.Same(id) {
			return true
		}
	}
	for _, b := range state.Burials {
		if b.Container != nil && b.Container.Same(id) {
			return true
		}
	}
	for _, p := range state.Proposals {
		for _, c := range p.Candidates {
			if c.Container != nil && c.Container.ID == id.ID {
				return true
			}
		}
	}
	return false
}

// Container metadata establishes ownership independently of the session cwd.
func projectContainer(row docker.Container) (string, string, error) {
	if dir := row.Labels["com.docker.compose.project.working_dir"]; dir != "" {
		if !filepath.IsAbs(dir) {
			return "", "", nil
		}
		root, err := pathutil.LongAbs(dir)
		return root, "compose working_dir", err
	}
	var root string
	for _, bind := range row.Binds {
		if !filepath.IsAbs(bind) {
			continue
		}
		p, found, err := project.Find(bind)
		if err != nil {
			return "", "", err
		}
		if !found {
			continue
		}
		if root != "" && core.ProjectKey(root) != core.ProjectKey(p.Root) {
			return "", "", nil
		}
		root = p.Root
	}
	return root, "bind mount in project", nil
}

func (e *Engine) approveContainer(p core.Proposal, c core.Candidate, class string) ([]core.Result, bool, error) {
	results, retry, err := e.Svc.ApproveContainer(p, c, class)
	if errors.Is(err, docker.ErrUnavailable) {
		err = errors.New(config.UILanguage().Text(i18n.StartDocker))
		for i := range results {
			results[i].Error = err.Error()
		}
	}
	return results, retry, err
}
