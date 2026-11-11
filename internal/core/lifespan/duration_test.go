package lifespan

import (
	"fmt"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func FuzzParseDuration(f *testing.F) {
	f.Fuzz(func(t *testing.T, text string) {
		d, err := ParseDuration(text)
		if err != nil {
			return
		}
		if d <= 0 || d > MaxLifespan || d%time.Second != 0 {
			t.Fatalf("accepted duration outside contract: %q -> %v", text, d)
		}
		again, err := ParseDuration(fmt.Sprintf("%ds", d/time.Second))
		if err != nil || again != d {
			t.Fatalf("duration round trip: %v -> %v, %v", d, again, err)
		}
		if _, err := ParseDuration(fmt.Sprintf("%ds1s", MaxLifespan/time.Second)); err == nil {
			t.Fatal("sum above the cap accepted")
		}
	})
}

func FuzzParseAt(f *testing.F) {
	loc := time.FixedZone("CST", 8*3600)
	f.Fuzz(func(t *testing.T, text string, seconds int64) {
		// Bound the clock to ordinary calendar years, independently of the input.
		now := time.Date(2026, 11, 7, 20, 0, 0, 0, loc).Add(time.Duration(seconds%(366*24*3600)) * time.Second)
		at, err := ParseAt(text, now)
		if err != nil {
			return
		}
		if !at.After(now) || at.Sub(now) > MaxLifespan || at.Second() != 0 || at.Nanosecond() != 0 || at.Location() != loc {
			t.Fatalf("invalid expiry: %q, now=%v, at=%v", text, now, at)
		}
		again, err := ParseAt(at.Format("2006-01-02 15:04"), now)
		if err != nil || !again.Equal(at) {
			t.Fatalf("expiry round trip: %v -> %v, %v", at, again, err)
		}
		if clock, err := time.Parse("15:04", strings.TrimSpace(text)); err == nil {
			if at.Sub(now) > 24*time.Hour || at.Hour() != clock.Hour() || at.Minute() != clock.Minute() {
				t.Fatalf("clock did not select its next occurrence: %q -> %v", text, at)
			}
		}
	})
}

func TestParseDuration(t *testing.T) {
	valid := map[string]time.Duration{
		"90s":    90 * time.Second,
		"30m":    30 * time.Minute,
		"2h":     2 * time.Hour,
		"1d12h":  36 * time.Hour,
		"2W":     14 * 24 * time.Hour,
		" 1h30m": 90 * time.Minute,
	}
	for in, want := range valid {
		got, err := ParseDuration(in)
		if err != nil || got != want {
			t.Errorf("ParseDuration(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "2", "h", "2x", "0m", "-1h", "1h-", "99999999999w", "3651d"} {
		if _, err := ParseDuration(in); err == nil {
			t.Errorf("ParseDuration(%q) succeeded; want an error", in)
		}
	}
}

func TestParseAt(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 11, 7, 20, 0, 0, 0, loc)
	cases := map[string]time.Time{
		"2026-11-08 09:30": time.Date(2026, 11, 8, 9, 30, 0, 0, loc),
		"2026-11-09":       time.Date(2026, 11, 9, 0, 0, 0, 0, loc),
		"21:15":            time.Date(2026, 11, 7, 21, 15, 0, 0, loc),
		"08:00":            time.Date(2026, 11, 8, 8, 0, 0, 0, loc), // already passed today
	}
	for in, want := range cases {
		got, err := ParseAt(in, now)
		if err != nil || !got.Equal(want) {
			t.Errorf("ParseAt(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"2026-11-07 19:00", "yesterday", "2040-01-01"} {
		if _, err := ParseAt(in, now); err == nil {
			t.Errorf("ParseAt(%q) succeeded; want an error", in)
		}
	}
}

func TestFormatRemaining(t *testing.T) {
	cases := map[time.Duration]string{
		-time.Second:                "due",
		45 * time.Second:            "45s",
		12 * time.Minute:            "12m",
		2*time.Hour + 5*time.Minute: "2h 5m",
		3 * time.Hour:               "3h",
		28 * time.Hour:              "1d 4h",
		14 * 24 * time.Hour:         "14d",
	}
	for in, want := range cases {
		if got := FormatRemaining(in); got != want {
			t.Errorf("FormatRemaining(%v) = %q; want %q", in, got, want)
		}
	}
}

func TestParseDurationCannotOverflowPastTheCap(t *testing.T) {
	// 58 × 3650d plus this many seconds wraps int64 to about 290ms without a running check.
	in := strings.Repeat("3650d", 58) + "155864074s"
	if d, err := ParseDuration(in); err == nil {
		t.Fatalf("ParseDuration(overflowing sum) = %v; want an error", d)
	}
}

func TestLifespanDoesNotImportCore(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range file.Imports {
			name, _ := strconv.Unquote(imp.Path.Value)
			if name == "github.com/woodfishhhh/obituary/internal/core" {
				t.Errorf("%s: lifespan must not import the core facade", path)
			}
		}
	}
}
