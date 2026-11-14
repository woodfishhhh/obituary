package hook

import (
	"strings"

	"github.com/woodfishhhh/obituary/internal/auto"
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
	receipts    []int64      // registrations made without asking, told here
	cleanups    []cleanupAck // earlier sessions' cleanup requests told here
	cont        bool         // Stop: ask the agent to continue with the text
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
	for _, c := range d.cleanups {
		if _, err := (&auto.Engine{Svc: d.svc}).MarkCleanup(c.root, c.session, auto.CleanupTold); err != nil {
			Logf(d.svc.Store.Home, "cleanup acknowledgement: %v", err)
		}
	}
	if len(d.receipts) > 0 {
		if err := d.svc.MarkReceiptsTold(d.receipts); err != nil {
			Logf(d.svc.Store.Home, "receipt acknowledgement: %v", err)
		}
	}
	if len(d.ids) == 0 {
		return
	}
	if _, err := d.svc.MarkNotifiedBatch(d.ids, session); err != nil {
		// Already delivered: a failed acknowledgement must never retract text.
		Logf(d.svc.Store.Home, "notification acknowledgement: %v", err)
	}
}
