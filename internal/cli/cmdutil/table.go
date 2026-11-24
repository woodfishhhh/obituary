package cmdutil

import (
	"io"

	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/texttable"
)

// Table lays out tab-separated rows; see texttable.
func Table(out io.Writer, language i18n.Language) texttable.Writer {
	return texttable.New(out, language)
}

// Column pads translated cells by terminal cells; see texttable.
func Column(l *i18n.Localizer, text string) any { return texttable.Column(l, text) }
