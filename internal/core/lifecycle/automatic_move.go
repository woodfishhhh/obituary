package lifecycle

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

const gitMoveFreshness = 10 * time.Millisecond
const ownerQueryBatchSize = 128
const defaultGitVerificationBudget = 250 * time.Millisecond

var errOwnerSnapshotRefresh = errors.New("owner snapshot needs an outside-lock refresh")

func gitVerificationBudget(elapsed time.Duration) time.Duration {
	// Freshness dates from the Git reply or start of final ownership validation.
	// Bound lock contention separately, including slow uncached Git queries.
	return max(defaultGitVerificationBudget, 2*min(elapsed, time.Second))
}

func (s *Service) verificationBudget(item Item, real string, owners *ownershipBatch) (time.Duration, error) {
	var elapsed time.Duration
	if owners != nil && !owners.retried[item.ID] {
		elapsed = owners.elapsed[owners.groups[real]]
	}
	budget := gitVerificationBudget(elapsed)
	if owners == nil || budget <= defaultGitVerificationBudget {
		return budget, nil
	}
	rootCtx, cancel := context.WithTimeout(s.Store.Context(), defaultGitVerificationBudget)
	sameRoot, err := owners.currentRoot(rootCtx, real)
	cancel()
	if err != nil {
		return defaultGitVerificationBudget, nil
	}
	if sameRoot {
		return budget, nil
	}
	if owners.retried[item.ID] {
		return defaultGitVerificationBudget, nil
	}
	return 0, owners.requestRefresh(item.ID, real).Err
}

// Git's index may be atomically published during an earlier move, even within
// 10 ms. Reusing an answer requires a fresh proof of the same index generation.
// Call only after Guard, Rev, child lifespans, manifest writes and rules finish.
func (s *Service) finalGitForMove(item Item, real string, owners *ownershipBatch) (action, detail string, err error) {
	if s.OwnerBatch == nil && s.Owner == nil {
		return "", "", nil
	}
	budget, err := s.verificationBudget(item, real, owners)
	if err != nil {
		return "", "", err
	}
	ctx, cancel := context.WithTimeout(s.Store.Context(), budget)
	defer cancel()
	for {
		if err := ctx.Err(); err != nil {
			return ActionRetry, "final Git verification budget exhausted: " + err.Error(), nil
		}
		started := time.Now()
		var owner Ownership
		if owners != nil {
			owner = owners.lookup(ctx, item.ID, real)
		} else if s.OwnerBatch != nil {
			owner = s.OwnerBatch(ctx, []string{real})[0]
		} else {
			owner.Root, owner.Tracked, owner.Err = s.Owner(real)
			owner.CheckedAt = time.Now()
		}
		if item.Source == "watch" && owner.CheckedAt.After(started) {
			// Folder-policy freshness covers the whole final Git/index check.
			owner.CheckedAt = started
		}
		if errors.Is(owner.Err, errOwnerSnapshotRefresh) {
			return "", "", owner.Err
		}
		if action, detail := automaticOwnerVerdict(item, owner); action != "" {
			return action, detail, nil
		}
		if err := s.burialStep("git-checked"); err != nil {
			return "", "", err
		}
		// A stalled caller cannot spend an arbitrarily old permission to move.
		// The check is adjacent to the return to MovePinned; OS scheduling and
		// the single in-flight pinned operation are not hard real-time bounds.
		if time.Since(owner.CheckedAt) <= gitMoveFreshness {
			return "", "", nil
		}
	}
}

// The cached answers belong to one sweep. Query up to 128 candidates at once,
// independently of the 32-item ledger transactions. Every use passes Current
// after the item's final checks. Providers without Current remain per-move.
type ownershipBatch struct {
	answers map[string]Ownership
	paths   map[int64]string
	groups  map[string]string
	done    map[int64]bool
	pending map[string][]int64
	query   OwnerBatchFunc
	elapsed map[string]time.Duration
	refresh string
	retried map[int64]bool
}

func (s *Service) prepareBatchOwners(items []Item, blocked map[int64]bool) *ownershipBatch {
	if s.OwnerBatch == nil && s.PrepareOwners == nil {
		return nil
	}
	b := &ownershipBatch{
		answers: make(map[string]Ownership), paths: make(map[int64]string), done: make(map[int64]bool),
		groups: make(map[string]string), pending: make(map[string][]int64), elapsed: make(map[string]time.Duration),
		retried: make(map[int64]bool), query: s.OwnerBatch,
	}
	var paths []string
	for _, item := range items {
		if automatic(item) && item.Identity != nil && item.Due(s.now()) && !blocked[item.ID] {
			if path, err := s.FS.RealPath(item.Path); err == nil {
				b.paths[item.ID] = path
				paths = append(paths, path)
			}
		}
	}
	if s.PrepareOwners != nil {
		s.OwnerBatch = s.PrepareOwners(s.Store.Context(), paths, b.groups)
		b.query = s.OwnerBatch
	}
	for _, item := range items {
		if path, ok := b.paths[item.ID]; ok {
			group := ownerGroup(path, b.groups[path], item.IsDir)
			b.groups[path] = group
			b.pending[group] = append(b.pending[group], item.ID)
		}
	}
	return b
}

// Discovery and RealPath return canonical names. A folder without a worktree
// shares a budget only with other candidates from that same directory.
func ownerGroup(path, root string, isDir bool) string {
	if root == "" {
		root = path
		if !isDir {
			root = filepath.Dir(path)
		}
	}
	return pathutil.ComparisonPath(filepath.Clean(root))
}

// A query's elapsed time belongs only to the worktree whose paths it checked.
func (b *ownershipBatch) batchPaths(real string) []string {
	paths := []string{real}
	for _, id := range b.pending[b.groups[real]] {
		if len(paths) >= ownerQueryBatchSize {
			break
		}
		if path := b.paths[id]; !b.done[id] && path != real {
			paths = append(paths, path)
		}
	}
	return paths
}

func (b *ownershipBatch) fill(ctx context.Context, paths []string) {
	started := time.Now()
	queried := b.query(ctx, paths)
	elapsed := time.Since(started)
	groups := make([]string, len(paths))
	mixed := false
	for i, path := range paths {
		b.answers[path] = queried[i]
		group := b.groups[path]
		if root := queried[i].Root; root != "" {
			group = pathutil.ComparisonPath(filepath.Clean(root))
			b.groups[path] = group
		}
		groups[i] = group
		if i > 0 && group != groups[0] {
			mixed = true
		}
	}
	if mixed {
		return
	}
	for _, group := range groups {
		b.elapsed[group] = max(b.elapsed[group], elapsed)
	}
}

func (b *ownershipBatch) prefetch(ctx context.Context, pending []*preparedBurial) {
	if b.refresh != "" {
		b.groups[b.refresh] = ownerGroup(b.refresh, "", pending[0].out.Item.IsDir)
		b.fill(ctx, []string{b.refresh})
		b.refresh = ""
		return
	}
	for _, p := range pending {
		if p.out.Action != "" || p.moved || !automatic(p.out.Item) {
			continue
		}
		path := b.paths[p.out.Item.ID]
		if _, ok := b.answers[path]; path != "" && !ok {
			b.fill(ctx, b.batchPaths(path))
		}
	}
}

// Yield both locks once per item, including when its resolved path changes.
// Continued index churn keeps the file and its registration for a later sweep.
func (b *ownershipBatch) requestRefresh(id int64, real string) Ownership {
	if b.retried[id] {
		return Ownership{Tracked: true, Err: errors.New("repository snapshot changed again after refresh")}
	}
	b.retried[id], b.refresh = true, real
	return Ownership{Tracked: true, Err: errOwnerSnapshotRefresh}
}

func (b *ownershipBatch) currentRoot(ctx context.Context, real string) (bool, error) {
	owner, ok := b.answers[real]
	if !ok || owner.CurrentRoot == nil {
		return true, nil
	}
	return owner.CurrentRoot(ctx)
}

func (b *ownershipBatch) lookup(ctx context.Context, id int64, real string) Ownership {
	owner, ok := b.answers[real]
	if !ok {
		return b.requestRefresh(id, real)
	}
	if owner.Err != nil || owner.Current == nil {
		// An uncacheable answer has no generation proof; only a live final query is safe.
		fresh := b.query(ctx, []string{real})[0]
		b.answers[real] = fresh
		owner = fresh
		if owner.Err != nil || owner.Current == nil {
			return owner
		}
	}
	if err := ctx.Err(); err != nil {
		return Ownership{Tracked: true, Err: err}
	}
	current, checkedAt, err := owner.Current(ctx)
	owner.CheckedAt = checkedAt
	if err != nil {
		owner.Tracked, owner.Err = true, err
		return owner
	}
	if !current {
		return b.requestRefresh(id, real)
	}
	if time.Since(checkedAt) > gitMoveFreshness {
		owner.Tracked, owner.Err = true, context.DeadlineExceeded
	}
	return owner
}
