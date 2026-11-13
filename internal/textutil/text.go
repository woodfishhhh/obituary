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

// Bytes formats a size with binary units, such as "1.5 MiB".
func Bytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
