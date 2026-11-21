package cmdutil

import (
	"bytes"
	"fmt"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"strings"
	"testing"
)

func TestTableAndActionTerminalWidths(t *testing.T) {
	for _, tc := range []struct {
		lang        i18n.Language
		table, cell string
	}{{i18n.Chinese, "路径  备注\na     b\n", "对象    |"}, {i18n.English, "路径  备注\na   b\n", "对象      |"}} {
		var out bytes.Buffer
		w := Table(&out, tc.lang)
		fmt.Fprint(w, "路径\t备注\na\tb\n")
		if err := w.Flush(); err != nil || out.String() != tc.table {
			t.Fatalf("%s: %q %v", tc.lang, out.String(), err)
		}
		got := fmt.Sprintf("%-8s|", Column(i18n.NewLocalizer(tc.lang), "对象"))
		if got != tc.cell {
			t.Fatal(got)
		}
	}
	if got := fmt.Sprintf("%-3s", wideCell("é")); strings.TrimSuffix(got, "  ") != "é" {
		t.Fatal(got)
	}
}
