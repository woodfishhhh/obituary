package auto

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"github.com/woodfishhhh/obituary/internal/project"
)

// The session cleanup ([hooks] session_cleanup) asks an agent, once per
// session, to sort the files it created that nothing else accounted for. It
// works from the same evidence as tiering: paths the hook saw the agent write
// (recorded per project and session next to the session snapshot), the
// snapshot, and creation times. A file that existed before the session, that
// Git tracks, that has a lifespan or that waits in a proposal is never listed.

// CleanupFile is one file the agent created without giving it a lifespan.
type CleanupFile struct {
	Path string
	Size int64
}

// Cleanup states, kept in the session's marker so the request is made once.
const (
	CleanupAsked   = "asked"   // the agent was asked when it tried to stop
	CleanupPending = "pending" // the agent cannot be asked; tell the next session
	CleanupTold    = "told"    // a later session start delivered it
)

type cleanupMarker struct {
	Project string `json:"project"`
	Session string `json:"session"`
	State   string `json:"state"`
}

func (e *Engine) sessionFile(root, session, ext string) string {
	return strings.TrimSuffix(e.snapshotPath(root, session), ".json") + ext
}

// RecordWrites remembers, per project and session, the paths a hook saw the
// agent write. Appends are small and independent, so parallel hooks of one
// session cannot lose each other's lines.
func (e *Engine) RecordWrites(rows []core.Observation) error {
	byFile := map[string][]string{}
	for _, row := range rows {
		if row.Project == "" || row.Session == "" {
			continue
		}
		root, err := pathutil.LongAbs(row.Project)
		if err != nil {
			continue
		}
		file := e.sessionFile(root, row.Session, ".writes")
		byFile[file] = append(byFile[file], row.Path)
	}
	var errs error
	for file, paths := range byFile {
		errs = errors.Join(errs, appendLines(file, paths))
	}
	return errs
}

func appendLines(file string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, err = f.WriteString(strings.Join(lines, "\n") + "\n")
	return errors.Join(err, f.Close())
}

func readLines(file string) ([]string, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	seen := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := filepath.Clean(strings.TrimSpace(scanner.Text()))
		if key := strings.ToLower(line); line != "." && !seen[key] {
			seen[key] = true
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}

// CleanupCandidates lists what the session's agent created in the project and
// left without a lifespan: written by the agent according to the hook, absent
// from the session snapshot and created after it started, untracked by Git,
// not covered by a lifespan or a pending proposal, and still there.
func (e *Engine) CleanupCandidates(root, session string) ([]CleanupFile, error) {
	root, err := pathutil.LongAbs(root)
	if err != nil {
		return nil, err
	}
	written, err := readLines(e.sessionFile(root, session, ".writes"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	snap, err := e.readSnapshot(e.snapshotPath(root, session))
	if err != nil || !snap.Git {
		return nil, nil // nothing provably created: nothing to ask about
	}
	st, err := e.Svc.Store.ReadHot()
	if err != nil {
		return nil, err
	}
	var files []CleanupFile
	var paths []string
	for _, path := range written {
		info, err := os.Lstat(path)
		if err != nil || accounted(st, root, path) || !createdIn(snap, root, path) {
			continue
		}
		files = append(files, CleanupFile{Path: path, Size: info.Size()})
		paths = append(paths, path)
	}
	if len(paths) == 0 {
		return nil, nil
	}
	tracked, err := project.Project{Root: root}.TrackedBatch(paths)
	if err != nil {
		return nil, err // Git cannot say: ask about nothing rather than tracked files
	}
	var out []CleanupFile
	for i, f := range files {
		if !tracked[i] {
			out = append(out, f)
		}
	}
	return out, nil
}

// accounted reports a path that already has a lifespan (itself or a folder
// above it), waits in a proposal, or lies in Obituary's own work directory.
func accounted(st core.State, root, path string) bool {
	covers := func(p string) bool {
		return strings.EqualFold(filepath.Clean(p), path) || inside(strings.ToLower(path), strings.ToLower(filepath.Clean(p)))
	}
	if covers(project.Project{Root: root}.WorkDir()) {
		return true
	}
	for _, it := range st.Items {
		if it.Kind != core.KindContainer && covers(it.Path) {
			return true
		}
	}
	for _, p := range st.Proposals {
		if slices.ContainsFunc(p.Candidates, func(c core.Candidate) bool { return c.Kind != core.KindContainer && covers(c.Path) }) {
			return true
		}
	}
	return false
}

// CleanupState returns the session's cleanup state ("" before any).
func (e *Engine) CleanupState(root, session string) string {
	m, err := e.readCleanup(root, session)
	if err != nil {
		return ""
	}
	return m.State
}

func (e *Engine) readCleanup(root, session string) (cleanupMarker, error) {
	var m cleanupMarker
	root, err := pathutil.LongAbs(root)
	if err != nil {
		return m, err
	}
	data, err := os.ReadFile(e.sessionFile(root, session, ".cleanup"))
	if err != nil {
		return m, err
	}
	return m, json.Unmarshal(data, &m)
}

// MarkCleanup records the session's cleanup state. Asked and pending are
// claimed exclusively, so of two racing ending events only one asks.
func (e *Engine) MarkCleanup(root, session, state string) (bool, error) {
	root, err := pathutil.LongAbs(root)
	if err != nil {
		return false, err
	}
	file := e.sessionFile(root, session, ".cleanup")
	data, err := json.Marshal(cleanupMarker{Project: root, Session: session, State: state})
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return false, err
	}
	if state == CleanupTold {
		return true, os.WriteFile(file, data, 0o600)
	}
	f, err := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, os.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = f.Write(data)
	return true, errors.Join(err, f.Close())
}

// PendingCleanups lists sessions in the project whose agent could not be
// asked when it stopped; the next session start in the project hears of them.
func (e *Engine) PendingCleanups(root string) []string {
	root, err := pathutil.LongAbs(root)
	if err != nil {
		return nil
	}
	prefix := strings.TrimSuffix(filepath.Base(e.snapshotPath(root, "x")), "x.json")
	matches, _ := filepath.Glob(filepath.Join(e.Svc.Store.Home, "sessions", prefix+"*.cleanup"))
	var sessions []string
	for _, file := range matches {
		var m cleanupMarker
		data, err := os.ReadFile(file)
		if err != nil || json.Unmarshal(data, &m) != nil || m.State != CleanupPending || !strings.EqualFold(m.Project, root) {
			continue
		}
		sessions = append(sessions, m.Session)
	}
	return sessions
}
