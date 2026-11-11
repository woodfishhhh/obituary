package guardrm

import (
	"fmt"
	"strings"
	"testing"
)

func TestBashResidualContinuationsPassThrough(t *testing.T) {
	t.Parallel()
	for _, count := range []int{3, 5, 7, 9} {
		for _, operand := range []string{"./", "a/..", "keep"} {
			for _, quote := range []string{"", `"`} {
				t.Run(fmt.Sprintf("slashes-%d/%s/quote-%q", count, operand, quote), func(t *testing.T) {
					t.Parallel()
					command := "rm -f victim " + quote + operand + strings.Repeat("\\", count) + "\n" + quote
					got, why := Suggest(command, Context{Shell: Bash})
					if got != command || why == "" {
						t.Fatalf("residual Bash continuation received a suggestion: %q -> %q (%s)", command, got, why)
					}
					equivalent, _, summaryWhy := SuggestWithSummary(command, Context{Shell: Bash})
					if equivalent != command || summaryWhy == "" {
						t.Fatalf("residual Bash continuation received a summary suggestion: %q -> %q (%s)", command, equivalent, summaryWhy)
					}
				})
			}
		}
	}
}

func TestBashSimpleContinuations(t *testing.T) {
	t.Parallel()
	commands := []string{"rm \\\n-f x", "rm -- ke\\\nep", "rm -- \"ke\\\nep\""}
	for _, count := range []int{2, 4, 6, 8} {
		for _, quote := range []string{"", `"`} {
			commands = append(commands, "rm -- "+quote+"keep"+strings.Repeat("\\", count)+"\n"+quote)
		}
	}
	for _, command := range commands {
		if got, why := Suggest(command, Context{Shell: Bash}); got != "obit "+command || why != "" {
			t.Errorf("simple Bash continuation or even backslash run lost: %q -> %q (%s)", command, got, why)
		}
	}
}
