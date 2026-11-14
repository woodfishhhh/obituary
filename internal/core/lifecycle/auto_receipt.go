package lifecycle

import "slices"

// MarkReceiptsTold records that an agent session has been told about these
// registrations; each receipt is announced to one session only.
func (s *Service) MarkReceiptsTold(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return s.Store.UpdateHot(func(st *State) error {
		for i := range st.AutoReceipts {
			if slices.Contains(ids, st.AutoReceipts[i].ID) {
				st.AutoReceipts[i].Told = true
			}
		}
		return nil
	})
}

// forgetAutoReceipt drops a spared item's receipt. A container the session
// started is remembered as declined, so the next session finish does not
// register it again (files keep their spare exemption instead).
func (s *Service) forgetAutoReceipt(st *State, it Item) {
	r, ok := st.AutoReceipt(it.ID)
	if !ok {
		return
	}
	st.AutoReceipts = slices.DeleteFunc(st.AutoReceipts, func(old AutoReceipt) bool { return old.ID == it.ID })
	if it.Kind == KindContainer && it.Container != nil {
		rememberRejectedLeftovers(st, Proposal{Project: r.Project, Session: r.Session,
			Candidates: []Candidate{{Kind: KindContainer, Container: it.Container, Path: it.Path}}}, s.now())
	}
}
