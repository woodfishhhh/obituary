package hook

import (
	"strings"

	"github.com/woodfishhhh/obituary/internal/core"
)

// A delivery contains only complete summaries that fit the hook budget. The
// caller writes it before acknowledging; failed output leaves notices pending.
type delivery struct {
	graveUnlock func()
	deny        bool
	skipped     bool
	graves      []int64
	graveHome   string
	graveEvent  Event
	lines       []string
	ids         []int64
	svc         *core.Service
}

func (d *delivery) append(p core.Proposal, consent string) {
	var fits bool
	d.lines, fits = appendContext(d.lines, describeProposal(p, consent))
	if fits {
		d.ids = append(d.ids, p.ID)
	}
}

func (d delivery) close() {
	if d.graveUnlock != nil {
		d.graveUnlock()
	}
}

func (d delivery) text() string { return capContext(strings.Join(d.lines, "\n")) }

func (d delivery) acknowledge(session string) {
	if len(d.graves) > 0 {
		writeGraveSession(d.graveHome, d.graveEvent, nil, d.graves)
	}
	if len(d.ids) == 0 {
		return
	}
	if _, err := d.svc.MarkNotifiedBatch(d.ids, session); err != nil {
		// Already delivered: a failed acknowledgement must never retract text.
		Logf(d.svc.Store.Home, "notification acknowledgement: %v", err)
	}
}
