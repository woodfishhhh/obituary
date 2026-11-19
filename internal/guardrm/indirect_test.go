package guardrm

import "testing"

func TestBashIndirectDeletion(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ command, want string }{
		{`find . -name x -delete`, "find -delete"},
		{`find . -exec rm -rf {} +`, "find -exec rm"},
		{`find . -exec rm {} \;`, "find -exec rm"},
		{`find . -execdir '/usr/bin/rm' '{}' +`, "find -execdir rm"},
		{`find . -exec echo {} + -o -delete`, "find -delete"},
		{`find . -exec echo {} + -exec rm {} \;`, "find -exec rm"},
		{`find . -exec echo -delete ';' -delete`, "find -delete"},
		{`find . -print0 | xargs rm -rf`, "xargs rm"},
		{`xargs -0 rm`, "xargs rm"},
		{`find . -print0 | /usr/bin/xargs -0rn1 -- 'rm.exe'`, "xargs rm"},
		{`xargs -0I '{}' --arg-file file --max-procs=2 rmdir '{}'`, "xargs rmdir"},
		{`find . -name '*.go'`, ""},
		{`find . -type f -print`, ""},
		{`find . -name '-delete'`, ""},
		{`find . -name '-exec' -o -printf '-delete'`, ""},
		{`find . -exec echo rm -delete \;`, ""},
		{`find . -exec rm --help \;`, ""},
		{`echo find . -delete`, ""},
		{`echo xargs rm`, ""},
		{`xargs -0 printf '%s\n' rm`, ""},
		{`xargs -a rm`, ""},
		{`xargs --arg-file rm printf`, ""},
		{`xargs -I rm echo rm`, ""},
		{`xargs --help rm`, ""},
		{`xargs rm --help`, ""},
		{"find . -delete\nfi", "find -delete"},
	} {
		t.Run(tc.command, func(t *testing.T) {
			t.Parallel()
			if got := Deletion(tc.command, Bash); got != tc.want {
				t.Errorf("Deletion = %q, want %q", got, tc.want)
			}
		})
	}
}
