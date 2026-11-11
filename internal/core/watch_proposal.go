package core

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"
)

const ProposalWatchBurial = "watch-burial"
const ActionProposed = "proposed"

// WatchBurialReview binds the decision to a lifespan and object, not a name.
// Counts and kept paths describe the displayed tree; final movement rechecks it.
type WatchBurialReview struct {
	ItemID   int64    `json:"itemId"`
	ItemRev  int64    `json:"itemRev"`
	Identity Identity `json:"identity"`
	Files    int64    `json:"files"`
	Kept     []string `json:"kept,omitempty"`
}

func (s *Service) proposeWatchBurial(out Outcome, dryRun bool) Outcome {
	st, err := s.Store.Read()
	if err != nil {
		return ledgerOutcome(out, err)
	}
	if err := s.checkWatchScope(&st, out.Item, out.Path, ReasonExpired); err != nil {
		return failedMove(out, err)
	}
	p, err := s.watchBurialProposal(&st, out.Item, out.Path)
	if err != nil {
		return failedMove(out, err)
	}
	out.Action, out.Detail = ActionProposed, p.Candidates[0].Rule
	if dryRun {
		return out
	}
	err = s.Store.Update(func(st *State) error {
		if _, err := s.burialRevision(st, out.Item, ReasonExpired); err != nil {
			return err
		}
		if err := s.checkWatchScope(st, out.Item, out.Path, ReasonExpired); err != nil {
			return err
		}
		for i, old := range st.Proposals {
			if old.Kind == ProposalWatchBurial && old.WatchBurial != nil && old.WatchBurial.ItemID == out.Item.ID {
				p.ID, p.CreatedAt, p.Notified = old.ID, old.CreatedAt, old.Notified
				if !reflect.DeepEqual(p, old) {
					st.Proposals[i] = p
				}
				return nil
			}
		}
		p.ID, p.CreatedAt = st.NextID, s.now()
		st.NextID++
		st.Proposals = append(st.Proposals, p)
		return nil
	})
	return ledgerOutcome(out, err)
}

func (s *Service) watchBurialProposal(st *State, it Item, path string) (Proposal, error) {
	r := &WatchBurialReview{ItemID: it.ID, ItemRev: it.Rev, Identity: *it.Identity}
	var size int64
	err := filepath.WalkDir(path, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := s.Store.Context().Err(); err != nil {
			return err
		}
		id, err := s.FS.Identify(current)
		if err != nil {
			return err
		}
		if st.watchExempt(it.WatchID, id) || st.automaticallyKept(id) {
			r.Kept = append(r.Kept, current)
		}
		if !entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			r.Files++
			size += info.Size()
		}
		return nil
	})
	rule := fmt.Sprintf("已到期，批准后下葬；%d 个文件；饶过或复活过的对象：无", r.Files)
	if len(r.Kept) > 0 {
		rule = fmt.Sprintf("已到期，批准后下葬；%d 个文件；包含饶过或复活过的对象：%s", r.Files, strings.Join(r.Kept, "、"))
	}
	return Proposal{Kind: ProposalWatchBurial, WatchID: it.WatchID, Project: it.Project, WatchBurial: r, CandidateSeq: uint64(it.Rev),
		Candidates: []Candidate{{Rev: uint64(it.Rev), Path: path, Lifespan: "expired", Size: size, Rule: rule}}}, err
}

func (st *State) automaticallyKept(id Identity) bool {
	for _, e := range st.AutoExemptions {
		if sameWatchIdentity(e.Identity, id) {
			return true
		}
	}
	return false
}

// Caller holds the existing proposal decision lock. Reuse the explicit burial
// transaction, adding watch scope/Git and post-move verification to that path.
func (s *Service) ApproveWatchBurial(p Proposal) (Outcome, error) {
	s, notify := s.trackChanges()
	defer notify()
	st, err := s.Store.Read()
	if err != nil {
		return Outcome{}, err
	}
	it, err := watchBurialItem(&st, p)
	if err != nil {
		return Outcome{}, err
	}
	tx := *s
	tx.approvedWatch, tx.Mode = &p, ModeGraveyard
	roots := tx.resolveGraveyardRoots(st, it.Path)
	out := tx.settle(it, false, ReasonBury, roots)
	if out.Action == ActionRetry {
		if err := tx.commit(out); err != nil {
			return out, err
		}
		return out, errors.Join(out.err, s.Store.RecordCommitted(Event{Time: s.now(), Action: out.Action, ID: it.ID, Path: out.Path, Detail: out.Detail}))
	}
	return out, out.err
}

func (s *Service) checkWatchBurialReview(st *State, p Proposal) error {
	it, err := watchBurialItem(st, p)
	if err != nil {
		return err
	}
	current, err := s.watchBurialProposal(st, it, p.Candidates[0].Path)
	if err != nil {
		return err
	}
	current.ID, current.CreatedAt, current.Notified = p.ID, p.CreatedAt, p.Notified
	if ProposalFingerprint(current) != ProposalFingerprint(p) {
		st.Proposals[st.ProposalIndex(p.ID)] = current
		return ErrProposalChanged
	}
	return nil
}

// Pending directory decisions have no timer deadline. A policy observation or
// lifespan edit invalidates the review; approval rechecks the displayed tree.
func (st *State) awaitingWatchBurial(it Item) bool {
	if it.Source != "watch" || !it.IsDir {
		return false
	}
	return slices.ContainsFunc(st.Proposals, func(p Proposal) bool {
		return p.Kind == ProposalWatchBurial && p.WatchBurial != nil && p.WatchBurial.ItemID == it.ID &&
			p.WatchID == it.WatchID && p.WatchBurial.ItemRev == it.Rev && it.Identity != nil && p.WatchBurial.Identity == *it.Identity
	})
}

func (st *State) dropWatchBurial(id int64) {
	st.Proposals = slices.DeleteFunc(st.Proposals, func(p Proposal) bool {
		return p.Kind == ProposalWatchBurial && p.WatchBurial != nil && p.WatchBurial.ItemID == id
	})
}

func watchBurialItem(st *State, p Proposal) (Item, error) {
	idx := st.ProposalIndex(p.ID)
	if p.Kind != ProposalWatchBurial || p.WatchBurial == nil || len(p.Candidates) != 1 || idx < 0 {
		return Item{}, ErrProposalChanged
	}
	if err := checkProposalReview(st.Proposals[idx], ProposalFingerprint(p)); err != nil {
		return Item{}, err
	}
	i := st.Index(p.WatchBurial.ItemID)
	if i < 0 {
		return Item{}, ErrProposalChanged
	}
	it := st.Items[i]
	if it.Rev != p.WatchBurial.ItemRev || it.Identity == nil || *it.Identity != p.WatchBurial.Identity || !it.IsDir || it.Source != "watch" || it.WatchID != p.WatchID {
		return Item{}, ErrProposalChanged
	}
	return it, nil
}

func (s *Service) watchMove(it Item, reason string) bool {
	return it.Source == "watch" && (reason == ReasonExpired || s.approvedWatch != nil)
}

func (st *State) rejectWatchBurial(p Proposal, at time.Time) {
	if p.WatchBurial == nil {
		return
	}
	i := st.Index(p.WatchBurial.ItemID)
	if i < 0 || st.Items[i].Identity == nil || *st.Items[i].Identity != p.WatchBurial.Identity {
		return
	}
	st.spareWatch(p.WatchID, st.Items[i].Identity, st.Items[i].Path, "rejected", at)
	st.Items = append(st.Items[:i], st.Items[i+1:]...)
}
