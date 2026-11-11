package core

import (
	"time"

	"github.com/woodfishhhh/obituary/internal/core/lifespan"
)

// MaxLifespan caps how far in the future an expiry may be set.
const MaxLifespan = lifespan.MaxLifespan

// ParseDuration parses lifespans such as "90s", "30m", "2h", "1d12h" or "2w".
// Units are s, m, h, d and w; every number needs a unit.
func ParseDuration(text string) (time.Duration, error) {
	return lifespan.ParseDuration(text)
}

// ParseAt parses an absolute expiry in local time: "2006-01-02 15:04",
// "2006-01-02" (midnight) or "15:04" (the next time that clock time occurs).
func ParseAt(text string, now time.Time) (time.Time, error) {
	return lifespan.ParseAt(text, now)
}

// FormatRemaining renders the time left as a short string such as "1d 4h" or "12m".
func FormatRemaining(left time.Duration) string {
	return lifespan.FormatRemaining(left)
}
