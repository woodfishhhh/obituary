package auto

import (
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/docker"
	"github.com/woodfishhhh/obituary/internal/pathutil"
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
	var candidates []core.Candidate
	for _, row := range rows {
		if row.Created.Before(snap.At) || row.Created.After(end) || knownContainer(state, row.Identity) {
			continue
		}
		reason, ok := projectContainer(row, root)
		if !ok {
			continue
		}
		id := row.Identity
		candidates = append(candidates, core.Candidate{Kind: core.KindContainer, Container: &id, Path: id.Display(), Lifespan: LeftoverLifespan, Reason: reason})
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	notice, err := e.propose(core.ProposalContainer, root, session, candidates, nil)
	if err != nil || notice == nil {
		return nil, err
	}
	return []Notice{*notice}, nil
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

// projectContainer ties a container to the session's project through its
// compose working directory or a bind mount inside the project. Only those are
// proposed: a container merely created during the session may be the user's
// own, started elsewhere, and is not one the agent can recommend stopping.
func projectContainer(row docker.Container, root string) (string, bool) {
	if dir := row.Labels["com.docker.compose.project.working_dir"]; dir != "" && inProject(root, dir) {
		return "compose working_dir", true
	}
	for _, bind := range row.Binds {
		if inProject(root, bind) {
			return "bind mount in project", true
		}
	}
	return "", false
}

func inProject(root, path string) bool {
	path, err := pathutil.LongAbs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
