package lifespan

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// MaxLifespan caps how far in the future an expiry may be set.
const MaxLifespan = 10 * 365 * 24 * time.Hour

var units = map[byte]time.Duration{
	's': time.Second,
	'm': time.Minute,
	'h': time.Hour,
	'd': 24 * time.Hour,
	'w': 7 * 24 * time.Hour,
}

// ParseDuration parses lifespans such as "90s", "30m", "2h", "1d12h" or "2w".
// Units are s, m, h, d and w; every number needs a unit.
func ParseDuration(text string) (time.Duration, error) {
	input := strings.ToLower(strings.TrimSpace(text))
	if input == "" {
		return 0, fmt.Errorf("empty duration")
	}
	var total time.Duration
	for rest := input; rest != ""; {
		digits := 0
		for digits < len(rest) && rest[digits] >= '0' && rest[digits] <= '9' {
			digits++
		}
		if digits == 0 || digits == len(rest) {
			return 0, fmt.Errorf("invalid duration %q: use forms like 30m, 2h, 1d12h", text)
		}
		unit, ok := units[rest[digits]]
		if !ok {
			return 0, fmt.Errorf("invalid duration %q: unknown unit %q (use s, m, h, d, w)", text, rest[digits])
		}
		count, err := strconv.ParseInt(rest[:digits], 10, 64)
		// Check against the room left so a long sum cannot overflow past the cap.
		if err != nil || count > int64((MaxLifespan-total)/unit) {
			return 0, fmt.Errorf("duration %q is longer than 10 years", text)
		}
		total += time.Duration(count) * unit
		rest = rest[digits+1:]
	}
	if total <= 0 {
		return 0, fmt.Errorf("duration %q must be positive", text)
	}
	if total > MaxLifespan {
		return 0, fmt.Errorf("duration %q is longer than 10 years", text)
	}
	return total, nil
}

// ParseAt parses an absolute expiry in local time: "2006-01-02 15:04",
// "2006-01-02" (midnight) or "15:04" (the next time that clock time occurs).
func ParseAt(text string, now time.Time) (time.Time, error) {
	input := strings.TrimSpace(text)
	loc := now.Location()
	for _, layout := range []string{"2006-01-02 15:04", "2006-01-02T15:04", "2006-01-02"} {
		if at, err := time.ParseInLocation(layout, input, loc); err == nil {
			return checkFuture(at, now, text)
		}
	}
	if clock, err := time.ParseInLocation("15:04", input, loc); err == nil {
		at := time.Date(now.Year(), now.Month(), now.Day(), clock.Hour(), clock.Minute(), 0, 0, loc)
		if !at.After(now) {
			// When clocks fall back the same wall time occurs twice; prefer its second occurrence today.
			if later := at.Add(time.Hour); later.After(now) && later.Hour() == at.Hour() && later.Minute() == at.Minute() {
				return later, nil
			}
			at = time.Date(now.Year(), now.Month(), now.Day()+1, clock.Hour(), clock.Minute(), 0, 0, loc)
		}
		return at, nil
	}
	return time.Time{}, fmt.Errorf("invalid time %q: use \"2006-01-02 15:04\", \"2006-01-02\" or \"15:04\"", text)
}

func checkFuture(at, now time.Time, text string) (time.Time, error) {
	if !at.After(now) {
		return time.Time{}, fmt.Errorf("time %q is not in the future", text)
	}
	if at.Sub(now) > MaxLifespan {
		return time.Time{}, fmt.Errorf("time %q is more than 10 years away", text)
	}
	return at, nil
}

// FormatRemaining renders the time left as a short string such as "1d 4h" or "12m".
func FormatRemaining(left time.Duration) string {
	if left <= 0 {
		return "due"
	}
	if left < time.Minute {
		return fmt.Sprintf("%ds", int(left.Seconds()+0.5))
	}
	minutes := int64(left.Round(time.Minute) / time.Minute)
	days, hours, mins := minutes/(24*60), minutes/60%24, minutes%60
	switch {
	case days > 0 && hours > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case days > 0:
		return fmt.Sprintf("%dd", days)
	case hours > 0 && mins > 0:
		return fmt.Sprintf("%dh %dm", hours, mins)
	case hours > 0:
		return fmt.Sprintf("%dh", hours)
	default:
		return fmt.Sprintf("%dm", mins)
	}
}
