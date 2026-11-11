package rules

import (
	"testing"
)

func set(t *testing.T, base, text string) Set {
	t.Helper()
	builtin, err := Parse(Builtin, "builtin")
	if err != nil {
		t.Fatal(err)
	}
	project, err := Parse(text, ".obitrules")
	if err != nil {
		t.Fatal(err)
	}
	return Set{Base: base, Rules: append(builtin, project...)}
}
