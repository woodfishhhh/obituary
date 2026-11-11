package core

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// settle decides and (unless dryRun) carries out what happens to one due item.
func (s *Service) settle(item Item, dryRun bool, reason string, roots []resolvedGraveyardRoot) Outcome {
	if reason == ReasonExpired && item.Source == "watch" {
		copy := *s
		copy.Mode = ModeGraveyard
		s = &copy
	}
	out := s.prepareSettlement(item, dryRun, reason, roots, nil)
	if out.Action != "" || out.err != nil {
		return out
	}
	if !s.recycleMode() {
		return s.moveToGrave(out, reason)
	}
	// Shell recycling is slow: hold the write lock only for the final check.
	// A later mutation may still race the Shell, but commit preserves its Rev.
	if err := s.Store.Update(func(st *State) error {
		if err := st.requireAuditSpace(); err != nil {
			return err
		}
		if _, err := s.burialRevision(st, item, reason); err != nil {
			return err
		}
		if err := s.checkChildLifespans(st, item, out.Path, reason); err != nil {
			out = failedMove(out, err)
			return err
		}
		return nil
	}); err != nil {
		if out.Action == ActionRetry {
			return out
		}
		return ledgerOutcome(out, err)
	}
	if err := s.FS.Recycle(out.Path, *item.Identity); err != nil {
		return failedMove(out, err)
	}
	out.Action = ActionRecycled
	return out
}

// A supplied snapshot avoids repeated JSON reads during batch preparation.
// It is never authority to move: burialRevision runs again under store.lock.
func (s *Service) prepareSettlement(item Item, dryRun bool, reason string, roots []resolvedGraveyardRoot, state *State) Outcome {
	batchSnapshot := state != nil
	out := Outcome{Item: item, Path: item.Path}
	if item.Identity == nil {
		out.Action, out.Detail = ActionRefused, "record has no file identity; add the path again"
		return out
	}
	target, moved, verdict, detail := s.locate(item)
	if verdict != "" {
		out.Action, out.Detail = verdict, detail
		return out
	}
	out.Path = target
	if state == nil {
		current, err := s.Store.Read()
		if err != nil {
			return ledgerOutcome(out, err)
		}
		state = &current
	}
	if s.insideGraveyard(roots, target) {
		out.Action, out.Detail = ActionRetry, "recovery warning: tracked object is in the graveyard without a verified manifest; see obit doctor"
		if !dryRun {
			out.err = s.Store.Record(Event{Time: s.now(), Action: "recovery warning", ID: item.ID, Path: target, Detail: out.Detail})
		}
		return out
	}
	if reason == ReasonExpired && automatic(item) && (dryRun || s.recycleMode()) {
		if verdict, detail := s.recheckAutomatic(item, target); verdict != "" {
			out.Action, out.Detail = verdict, detail
			return out
		}
	}
	if !batchSnapshot {
		// Batches check the complete Guard under store.lock in movePrepared.
		// Avoid resolving the same canonical path twice before one move.
		if err := s.Guard.Check(target, item.Force); err != nil {
			var refused *RefusedError
			if errors.As(err, &refused) {
				out.Action, out.Detail = ActionRefused, err.Error()
			} else {
				out.Action, out.Detail = ActionRetry, err.Error()
			}
			return out
		}
	}
	if moved {
		out.Detail = "moved from " + item.Path
	}
	if reason == ReasonExpired && item.Source == "watch" && item.IsDir {
		return s.proposeWatchBurial(out, dryRun)
	}
	done := ActionBuried
	if s.recycleMode() {
		done = ActionRecycled
	}
	if dryRun {
		if err := s.checkChildLifespans(state, item, target, reason); err != nil {
			return failedMove(out, err)
		}
		release, err := s.holdWatchForMove(state, item, target, reason)
		if err != nil {
			return failedMove(out, err)
		}
		release()
		out.Action = done
		return out
	}
	if !s.recycleMode() {
		if batchSnapshot {
			// The sweep already collected this snapshot's discoverable roots.
			// Rebuilding them for each item performs quadratic Win32 path work.
			out.retryCommand = fmt.Sprintf("the original obit bury command for %q", target)
			root := s.graveyardRoot(target)
			for _, known := range roots {
				if samePath(root, known.path) {
					out.retryCommand = "obit reap"
					break
				}
			}
		} else {
			out.retryCommand = s.pendingBurialRetry(*state, target)
		}
	}
	return out
}

// burialRevision runs in the transaction that moves the object. Rev == 0
// denotes an explicitly requested, reserved ID that has never been an Item.
func (s *Service) burialRevision(st *State, item Item, reason string) (int, error) {
	idx := st.Index(item.ID)
	if item.Rev == 0 {
		if idx >= 0 || st.FindIdentity(*item.Identity) >= 0 {
			return -1, errClaimChanged
		}
		return -1, nil
	}
	if idx < 0 || st.Items[idx].Rev != item.Rev || (reason == ReasonExpired && !st.Items[idx].Due(s.now())) {
		return -1, errClaimChanged
	}
	return idx, nil
}

func ledgerOutcome(out Outcome, err error) Outcome {
	out.err = err
	if err != nil && out.Grave != "" && out.Burial == nil {
		out.Action = ActionRecoveryPending
		command := out.retryCommand
		if command == "" {
			command = "obit reap"
		}
		out.Detail = fmt.Sprintf("ledger commit needs retry; release any file lock and rerun %s; object moved to %s: %v", command, out.Grave, err)
	}
	if errors.Is(err, errClaimChanged) {
		out.Action, out.Detail, out.err = ActionSkipped, err.Error(), nil
	}
	return out
}

func (s *Service) pendingBurialRetry(st State, path string) string {
	root := s.graveyardRoot(path)
	for _, known := range s.graveyardRoots(st) {
		if samePath(root, known) {
			return "obit reap"
		}
	}
	return fmt.Sprintf("the original obit bury command for %q", path)
}

func failedMove(out Outcome, err error) Outcome {
	out.Action, out.Detail, out.Grave, out.GraveID = ActionRetry, err.Error(), "", nil
	var refused *RefusedError
	if errors.Is(err, ErrUnsafe) || errors.As(err, &refused) {
		out.Action = ActionRefused
	}
	return out
}

// recheckAutomatic decides again, on the object's real location at expiry,
// what was decided when it was registered: it must still be in the same
// project, still untracked, and no rule may protect it (or anything in it).
func (s *Service) recheckAutomatic(item Item, target string) (verdict, detail string) {
	real, verdict, detail := s.recheckAutomaticRules(item, target)
	if verdict != "" {
		return verdict, detail
	}
	if s.Owner != nil {
		root, tracked, err := s.Owner(real)
		return automaticOwnerVerdict(item, Ownership{Root: root, Tracked: tracked, Err: err})
	}
	return "", ""
}

func (s *Service) recheckAutomaticRules(item Item, target string) (real, verdict, detail string) {
	real, err := s.FS.RealPath(target)
	if err != nil {
		return "", ActionRetry, "could not resolve its real location: " + err.Error()
	}
	if s.Recheck != nil && item.Project != "" {
		if err := s.Recheck(item.Project, real, item.IsDir); err != nil {
			if errors.Is(err, ErrProtected) {
				return "", ActionRefused, err.Error()
			}
			return "", ActionRetry, err.Error()
		}
	}
	return real, "", ""
}

func automaticOwnerVerdict(item Item, owner Ownership) (verdict, detail string) {
	switch {
	case owner.Err != nil:
		return ActionRetry, "could not ask Git whether it is tracked: " + owner.Err.Error()
	case !samePath(owner.Root, item.Project):
		return ActionRefused, "it is no longer inside " + item.Project + "; automatic lifespans stay within their project"
	case owner.Tracked:
		return ActionRefused, "Git tracks it now; it was committed after it got a lifespan"
	}
	return "", ""
}

func inRecycleBin(path string) bool {
	return isInside(path, filepath.Join(pathutil.VolumeRoot(path), "$Recycle.Bin"))
}

// locate finds the tracked object. verdict is non-empty when it must not be
// recycled now; only a definite "not found" lets an item be forgotten.
func (s *Service) locate(item Item) (target string, moved bool, verdict, detail string) {
	if err := pathutil.WholeObject(item.Path); err != nil {
		return "", false, ActionRetry, err.Error()
	}
	current, statErr := s.FS.Identify(item.Path)
	if statErr == nil && current == *item.Identity {
		return item.Path, false, "", ""
	}
	if statErr != nil && errors.Is(statErr, ErrUnsafe) {
		return "", false, ActionRefused, statErr.Error()
	}
	if statErr != nil && !errors.Is(statErr, ErrNotFound) {
		return "", false, ActionRetry, statErr.Error()
	}
	found, err := s.FS.Locate(*item.Identity)
	switch {
	case err == nil && inRecycleBin(found):
		return "", false, ActionVanished, "already in the Recycle Bin"
	case err == nil:
		return found, !samePath(found, item.Path), "", ""
	case errors.Is(err, ErrUnsafe):
		return "", false, ActionRefused, err.Error()
	case !errors.Is(err, ErrNotFound):
		return "", false, ActionRetry, err.Error()
	case statErr == nil:
		return "", false, ActionReplaced, "a different object now sits at this path"
	default:
		return "", false, ActionVanished, ""
	}
}
