// Package textutil shares small English display conventions across CLI and hooks.
package textutil

import (
	"fmt"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/lifespan"
)

// Count formats a count, optionally accepting an irregular plural.
func Count(n int, noun string, plural ...string) string {
	if n != 1 {
		if len(plural) > 0 {
			noun = plural[0]
		} else {
			noun += "s"
		}
	}
	return fmt.Sprintf("%d %s", n, noun)
}

func DueIn(left time.Duration) string {
	if left <= 0 {
		return "due"
	}
	return "in " + lifespan.FormatRemaining(left)
}
