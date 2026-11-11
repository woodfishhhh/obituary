package gui

import "testing"

func TestNewRequiresExplicitDependencies(t *testing.T) {
	a, _ := fixture(t)
	if _, err := New(a.deps); err != nil {
		t.Fatal(err)
	}
	for _, deps := range []Dependencies{{}, {Service: a.deps.Service, Engine: a.deps.Engine}} {
		if _, err := New(deps); err == nil {
			t.Fatal("incomplete dependencies accepted")
		}
	}
}
