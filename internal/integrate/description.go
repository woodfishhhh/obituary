package integrate

import (
	"fmt"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

// Store the original format separately: a path, JSON sample or block of agent
// instructions must never be searched for words to replace after interpolation.
type description struct {
	format string
	args   []any
}

func describe(format string, args ...any) *description { return &description{format, args} }
func (d *description) text(l *i18n.Localizer) string {
	f := l.HostText(d.format)
	if len(d.args) == 0 {
		return f
	}
	args := make([]any, len(d.args))
	copy(args, d.args)
	for i, a := range args {
		if nested, ok := a.(*description); ok {
			args[i] = nested.text(l)
		}
	}
	return fmt.Sprintf(f, args...)
}
func (c Change) described(d *description) Change { c.display = d; c.Detail = d.text(nil); return c }
func (c Change) DisplayDetail(l *i18n.Localizer) string {
	if c.display != nil && c.display.text(nil) == c.Detail {
		return c.display.text(l)
	}
	return l.HostText(c.Detail)
}
func (m *ManualSteps) DisplaySteps(l *i18n.Localizer) string {
	if m.display != nil {
		return m.display.text(l)
	}
	return l.HostText(m.Steps)
}

func (m *ManualSteps) Change(c Change) Change { c.display = m.display; c.Detail = m.Steps; return c }
func (c Change) appended(d *description) Change {
	first := c.display
	if first == nil || first.text(nil) != c.Detail {
		first = describe(c.Detail)
	}
	return c.described(describe("%s%s", first, d))
}
