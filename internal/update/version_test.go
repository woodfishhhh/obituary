package update

import "testing"

func TestNewer(t *testing.T) {
	for _, tc := range []struct {
		current, latest string
		newer           bool
	}{
		{"1.0.0", "1.0.1", true}, {"1.0.1", "1.0.1", false}, {"1.1.0", "1.0.1", false},
		{"1.9.0", "1.10.0", true}, {"v1.0.0", "v2.0.0", true},
		{"1.0.0-dev.abc", "1.0.0", false}, {"1.0.0-dev.abc", "1.0.1", true},
		{"1.1.0-dev.abc", "1.0.1", false}, {"0.0.0-dev", "1.0.0", true},
		{"1.0.0", "1.1.0-rc.1", false}, {"1.0.0+build", "1.0.0", false},
		{"unknown", "1.0.0", false}, {"1.0.0", "garbage", false},
	} {
		t.Run(tc.current+"/"+tc.latest, func(t *testing.T) {
			if got := Newer(tc.current, tc.latest); got != tc.newer {
				t.Fatalf("got %t want %t", got, tc.newer)
			}
		})
	}
}
