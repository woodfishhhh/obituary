package auto

import (
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/docker"
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
	if e.Svc.Docker == nil {
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
		owner, reason := containerOwner(row, root)
		id := row.Identity
		groups[owner] = append(groups[owner], core.Candidate{Kind: core.KindContainer, Container: &id, Path: id.Display(), Lifespan: LeftoverLifespan, Reason: reason})
	}
	var notices []Notice
	for owner, candidates := range groups {
		notice, err := e.propose(core.ProposalContainer, owner, session, candidates, nil)
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
	return false
}

func containerOwner(row docker.Container, root string) (string, string) {
	if dir := row.Labels["com.docker.compose.project.working_dir"]; dir != "" {
		if p, found, err := project.Find(dir); err == nil && found {
			return p.Root, "compose working_dir"
		}
	}
	for _, bind := range row.Binds {
		path, err := pathutil.LongAbs(bind)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(root, path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return root, "bind mount in project"
		}
	}
	return "", "会话期间新建"
}
