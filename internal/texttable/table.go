// Package texttable lays out the tab-separated columns of command output, counting
// terminal cells for translated text. It is a leaf so that both the commands and
// the resident can print the same table.
package texttable

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/rivo/uniseg"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

// Writer collects tab-separated rows and writes them aligned on Flush.
type Writer interface {
	io.Writer
	Flush() error
}

// New returns a Writer for out; non-English text is aligned by terminal cells.
func New(out io.Writer, language i18n.Language) Writer {
	if language == i18n.English {
		return tabwriter.NewWriter(out, 0, 4, 2, 32, 0)
	}
	return &cellTable{out: out}
}

type cellTable struct {
	strings.Builder
	out io.Writer
}

func (t *cellTable) Flush() error {
	rows := strings.Split(strings.TrimSuffix(t.String(), "\n"), "\n")
	widths := []int{}
	for _, row := range rows {
		cols := strings.Split(row, "\t")
		for i, c := range cols[:len(cols)-1] {
			if i == len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], uniseg.StringWidth(c))
		}
	}
	var out strings.Builder
	for _, row := range rows {
		cols := strings.Split(row, "\t")
		for i, c := range cols {
			out.WriteString(c)
			if i+1 < len(cols) {
				out.WriteString(strings.Repeat(" ", 2+widths[i]-uniseg.StringWidth(c)))
			}
		}
		out.WriteByte(10)
	}
	_, err := io.WriteString(t.out, out.String())
	return err
}

// Let translated printf formats keep their widths, counted in terminal cells.
type wideCell string

func (c wideCell) Format(s fmt.State, _ rune) {
	text := string(c)
	w, _ := s.Width()
	pad := strings.Repeat(" ", max(0, w-uniseg.StringWidth(text)))
	if s.Flag(45) {
		fmt.Fprint(s, text, pad)
	} else {
		fmt.Fprint(s, pad, text)
	}
}
func Column(l *i18n.Localizer, text string) any {
	if l.Language() == i18n.English {
		return text
	}
	return wideCell(text)
}
