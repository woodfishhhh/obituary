package ledger

import (
	"slices"
	"time"
)

// Classes of registrations made without a proposal (AutoReceipt.Class).
const (
	AutoCreated     = "created"     // a file the agent wrote, and created during its session
	AutoRegenerable = "regenerable" // a Git-ignored dependency, build output or cache the session left
	AutoContainer   = "container"   // a container the session started for its project
	AutoAll         = "auto"        // registered only because [proposals] mode is "auto"
)

// maxAutoReceipts bounds the hot ledger; receipts are informational only.
const maxAutoReceipts = 1000

// AutoReceipt records a registration Obituary made where it would once have
// proposed. The hook tells one agent session about it, and the window and
// `obit proposals` list it while the item is still tracked, so it can be undone.
type AutoReceipt struct {
	ID      int64     `json:"id"` // the registered item
	Class   string    `json:"class"`
	Project string    `json:"project,omitempty"`
	Session string    `json:"session,omitempty"`
	At      time.Time `json:"at"`
	Told    bool      `json:"told,omitempty"` // an agent session has been told
}

// AutoHandled is a receipt joined with its still-tracked item.
type AutoHandled struct {
	Item  Item      `json:"item"`
	Class string    `json:"class"`
	At    time.Time `json:"at"`
}

// AddAutoReceipts appends receipts, dropping those whose item is no longer
// tracked and then the oldest beyond the bound.
func (st *State) AddAutoReceipts(receipts ...AutoReceipt) {
	st.AutoReceipts = append(st.AutoReceipts, receipts...)
	st.AutoReceipts = slices.DeleteFunc(st.AutoReceipts, func(r AutoReceipt) bool { return st.Index(r.ID) < 0 })
	if extra := len(st.AutoReceipts) - maxAutoReceipts; extra > 0 {
		st.AutoReceipts = st.AutoReceipts[extra:]
	}
}

// AutoHandled lists the tracked items registered without asking, newest first.
func (st *State) AutoHandled() []AutoHandled {
	out := []AutoHandled{}
	for i := len(st.AutoReceipts) - 1; i >= 0; i-- {
		r := st.AutoReceipts[i]
		if idx := st.Index(r.ID); idx >= 0 {
			out = append(out, AutoHandled{Item: st.Items[idx], Class: r.Class, At: r.At})
		}
	}
	return out
}

// AutoReceipt returns the receipt of a tracked item, if it has one.
func (st *State) AutoReceipt(id int64) (AutoReceipt, bool) {
	for _, r := range st.AutoReceipts {
		if r.ID == id {
			return r, true
		}
	}
	return AutoReceipt{}, false
}
