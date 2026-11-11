package docker_test

import (
	"errors"
	"testing"

	"github.com/woodfishhhh/obituary/internal/docker"
)

func TestCheckRemoval(t *testing.T) {
	for _, tc := range []struct {
		name      string
		after     []string
		ambiguous bool
	}{
		{"target-only", []string{"neighbor"}, false},
		{"target-retained", []string{"target", "neighbor"}, false},
		{"added-container", []string{"neighbor", "new"}, false},
		{"neighbor-missing", nil, true},
		{"neighbor-only-missing", []string{"target"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := docker.CheckRemoval([]string{"target", "neighbor"}, tc.after, "target")
			if errors.Is(err, docker.ErrAmbiguousAction) != tc.ambiguous {
				t.Fatal("collateral removal detection", err)
			}
		})
	}
}
