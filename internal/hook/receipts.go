package hook

import (
	"fmt"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/textutil"
)

// classText says why an item went in without a proposal.
var classText = map[string]string{
	ledger.AutoCreated:     "created in its session",
	ledger.AutoRegenerable: "regenerable",
	ledger.AutoContainer:   "session container",
	ledger.AutoAll:         "auto mode",
}

// untoldReceipts lists registrations made without asking that no session has
// heard of yet: those in this project, or made for this session elsewhere.
func untoldReceipts(st ledger.State, root, session string, inProject bool) []ledger.AutoHandled {
	var out []ledger.AutoHandled
	for _, r := range st.AutoReceipts {
		mine := r.Session == session || inProject && ledger.ProjectKey(r.Project) == ledger.ProjectKey(root)
		if r.Told || !mine {
			continue
		}
		if idx := st.Index(r.ID); idx >= 0 {
			out = append(out, ledger.AutoHandled{Item: st.Items[idx], Class: r.Class, At: r.At})
		}
	}
	return out
}

// describeReceipts is the after-the-fact notice that replaces a proposal.
// Hook text is written as statements, not commands.
func describeReceipts(handled []ledger.AutoHandled, root string, now time.Time) string {
	var parts []string
	for i, h := range handled {
		if i == maxListed {
			parts = append(parts, fmt.Sprintf("and %d more (`obit proposals` lists them)", len(handled)-maxListed))
			break
		}
		parts = append(parts, fmt.Sprintf("#%d %s (%s, expires %s)", h.Item.ID, rel(root, h.Item.Path), classText[h.Class], textutil.DueIn(h.Item.ExpiresAt.Sub(now))))
	}
	return fmt.Sprintf("Obituary (讣告) registered %s automatically, without a proposal: %s. "+
		"Each is buried when its lifespan ends and stays restorable. `obit spare <id>` keeps one without a lifespan, "+
		"`obit extend <id> <lifespan>` keeps it longer, and `obit resurrect <id>` brings back one already buried.",
		textutil.Count(len(handled), "item"), strings.Join(parts, ", "))
}

func (d *delivery) appendReceipts(handled []ledger.AutoHandled, root string, now time.Time) {
	if len(handled) == 0 {
		return
	}
	var fits bool
	if d.lines, fits = appendContext(d.lines, describeReceipts(handled, root, now)); fits {
		for _, h := range handled {
			d.receipts = append(d.receipts, h.Item.ID)
		}
	}
}

func serviceNow(svc *lifecycle.Service) time.Time {
	if svc.Now != nil {
		return svc.Now()
	}
	return time.Now()
}
