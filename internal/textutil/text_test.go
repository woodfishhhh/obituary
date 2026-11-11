package textutil

import (
	"testing"
	"time"
)

func TestCount(t *testing.T) {
	for n, want := range []string{"0 items", "1 item", "2 items"} {
		if got := Count(n, "item"); got != want {
			t.Errorf("Count(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestCountIrregularPlural(t *testing.T) {
	for n, want := range []string{"0 policies", "1 policy", "2 policies"} {
		if got := Count(n, "policy", "policies"); got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	}
}

func TestDueIn(t *testing.T) {
	for _, tc := range []struct {
		left time.Duration
		want string
	}{
		{-time.Hour, "due"}, {0, "due"}, {time.Minute, "in 1m"},
	} {
		if got := DueIn(tc.left); got != tc.want {
			t.Errorf("DueIn(%s) = %q, want %q", tc.left, got, tc.want)
		}
	}
}
