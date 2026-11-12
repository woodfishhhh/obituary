package lifecycle

import (
	"context"
	"errors"
	"time"
)

const gitMoveFreshness = 10 * time.Millisecond
const ownerQueryBatchSize = 128

var errOwnerSnapshotRefresh = errors.New("owner snapshot needs an outside-lock refresh")

func gitVerificationBudget(elapsed time.Duration) time.Duration {
	// Freshness dates from the Git reply or start of final ownership validation.
	// Bound lock contention separately, including slow uncached Git queries.
	return max(250*time.Millisecond, 2*min(elapsed, time.Second))
}

// Git's index may be atomically published during an earlier move, even within
// 10 ms. Reusing an answer requires a fresh proof of the same index generation.
// Call only after Guard, Rev, child lifespans, manifest writes and rules finish.
func (s *Service) finalGitForMove(item Item, real string, lookup func(context.Context, string) Ownership) (action, detail string, err error) {
	if s.OwnerBatch == nil && s.Owner == nil {
		return "", "", nil
	}
	ctx, cancel := context.WithTimeout(s.Store.Context(), max(250*time.Millisecond, s.gitMoveBudget))
	defer cancel()
	for {
		if err := ctx.Err(); err != nil {
			return ActionRetry, "final Git verification budget exhausted: " + err.Error(), nil
		}
		started := time.Now()
		var owner Ownership
		if lookup != nil {
			owner = lookup(ctx, real)
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
	done    map[int64]bool
	pending []Item
	query   OwnerBatchFunc
	elapsed time.Duration
	refresh string
	retried map[string]bool
}

func (s *Service) prepareBatchOwners(items []Item, blocked map[int64]bool) *ownershipBatch {
	owners := s.batchOwners(items, blocked)
	if s.PrepareOwners != nil {
		var paths []string
		for _, item := range owners.pending {
			paths = append(paths, owners.paths[item.ID])
		}
		s.OwnerBatch = s.PrepareOwners(s.Store.Context(), paths)
		owners.query = s.OwnerBatch
	}
	return owners
}

func (s *Service) batchOwners(items []Item, blocked map[int64]bool) *ownershipBatch {
	if s.OwnerBatch == nil && s.PrepareOwners == nil {
		return nil
	}
	b := &ownershipBatch{answers: make(map[string]Ownership), paths: make(map[int64]string), done: make(map[int64]bool), query: s.OwnerBatch}
	// These names only select query candidates. Each item resolves its real
	// path again under store.lock, before consuming any cached answer.
	for _, item := range items {
		if automatic(item) && item.Identity != nil && item.Due(s.now()) && !blocked[item.ID] {
			if path, err := s.FS.RealPath(item.Path); err == nil {
				b.paths[item.ID] = path
				b.pending = append(b.pending, item)
			}
		}
	}
	return b
}

func (b *ownershipBatch) fill(ctx context.Context, real string) Ownership {
	var paths []string
	if real != "" {
		paths = append(paths, real)
	}
	for _, item := range b.pending {
		if len(paths) >= ownerQueryBatchSize {
			break
		}
		path := b.paths[item.ID]
		if !b.done[item.ID] && path != real {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return Ownership{}
	}
	started := time.Now()
	queried := b.query(ctx, paths)
	b.elapsed = max(b.elapsed, time.Since(started))
	for i, path := range paths {
		b.answers[path] = queried[i]
	}
	return queried[0]
}

func (b *ownershipBatch) prefetch(ctx context.Context, pending []*preparedBurial) {
	if b.refresh != "" {
		real := b.refresh
		b.refresh = ""
		b.fill(ctx, real)
		return
	}
	for _, p := range pending {
		if p.out.Action != "" || p.moved || !automatic(p.out.Item) {
			continue
		}
		path := b.paths[p.out.Item.ID]
		if _, ok := b.answers[path]; path != "" && !ok {
			b.fill(ctx, path)
		}
		return
	}
}

// One optimistic refresh yields both locks before running Git again. A second
// invalidation for this path falls back to the existing bounded locked query.
func (b *ownershipBatch) requestRefresh(real string) bool {
	if b.retried[real] {
		return false
	}
	if b.retried == nil {
		b.retried = make(map[string]bool)
	}
	b.retried[real], b.refresh = true, real
	return true
}

func (b *ownershipBatch) lookup(ctx context.Context, real string) Ownership {
	for {
		if err := ctx.Err(); err != nil {
			return Ownership{Tracked: true, Err: err}
		}
		if owner, ok := b.answers[real]; ok {
			if checked, done := b.lookupCachedOwner(ctx, real, owner); done {
				return checked
			}
			continue
		}
		owner := b.fill(ctx, real)
		if owner.Err != nil || owner.Current == nil {
			return owner
		}
		// A new answer is subject to the same final generation check.
	}
}

func (b *ownershipBatch) lookupCachedOwner(ctx context.Context, real string, owner Ownership) (Ownership, bool) {
	if owner.Err != nil || owner.Current == nil {
		fresh := b.query(ctx, []string{real})[0]
		b.answers[real] = fresh
		if fresh.Err != nil || fresh.Current == nil {
			return fresh, true
		}
		return Ownership{}, false
	}
	if owner.Current != nil {
		if checked, done := b.checkOwnerGeneration(ctx, real, owner); done {
			return checked, true
		}
	}
	owner = b.fill(ctx, real)
	if owner.Err != nil || owner.Current == nil {
		return owner, true
	}
	return Ownership{}, false
}

func (b *ownershipBatch) checkOwnerGeneration(ctx context.Context, real string, owner Ownership) (Ownership, bool) {
	current, checkedAt, err := owner.Current(ctx)
	owner.CheckedAt = checkedAt
	if err != nil {
		owner.Tracked, owner.Err = true, err
		return owner, true
	}
	if current {
		if time.Since(checkedAt) > gitMoveFreshness {
			owner.Tracked, owner.Err = true, context.DeadlineExceeded
		}
		return owner, true
	}
	if b.requestRefresh(real) {
		return Ownership{Tracked: true, Err: errOwnerSnapshotRefresh}, true
	}
	return Ownership{}, false
}
