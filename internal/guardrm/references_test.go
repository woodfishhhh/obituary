package guardrm

import (
	"reflect"
	"testing"
)

func TestReferencedPathsConservative(t *testing.T) {
	for _, tc := range []struct {
		shell   Shell
		command string
		want    []string
	}{
		{Bash, `cat 'old file.txt'`, []string{"old file.txt"}},
		{Bash, `ls -- old`, []string{"old"}},
		{PowerShell, `Get-Content -LiteralPath 'old file.txt'`, []string{"old file.txt"}},
		{PowerShell, `Get-ChildItem .\old`, []string{`.\old`}},
		{Bash, `echo old.txt`, nil}, {Bash, `cat "$FILE"`, nil}, {Bash, `cat *.txt`, nil}, {Bash, `cd other && cat old.txt`, nil},
		{Bash, `grep old.txt unrelated`, nil}, {Bash, `head -n 1 old.txt`, nil}, {PowerShell, `Get-Content $path`, nil}, {Uncertain, `cat old.txt`, nil},
	} {
		if got := ReferencedPaths(tc.command, tc.shell); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: %v != %v", tc.command, got, tc.want)
		}
	}
}
