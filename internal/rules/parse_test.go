package rules

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestRuleEncodingBoundary(t *testing.T) {
	for _, body := range []string{"\xef\xbb\xbf*.log off\n", "*.log off\n"} {
		parsed, err := Parse(body, FileName)
		if err != nil || len(parsed) != 1 || !parsed[0].re.MatchString("protected.log") || !parsed[0].Negate {
			t.Fatalf("UTF-8 BOM disabled protection: %+v %v", parsed, err)
		}
	}
	for _, body := range []string{"\xff*.log off\n", "\xff\xfe*\x00.\x00l\x00o\x00g\x00", "*\x00.\x00l\x00o\x00g\x00 \x00o\x00f\x00f\x00"} {
		if _, err := Parse(body, FileName); err == nil {
			t.Fatalf("invalid rule encoding accepted: %q", body)
		}
	}
}

func TestCharacterClassRangesDoNotMatchSeparators(t *testing.T) {
	t.Parallel()
	// FuzzRulePattern minimized this to " -0". The whitespace-free range
	// below also fits a real .obitrules field and used to match dir/file.log.
	for _, body := range []string{" -0", ".-0", "\x00-\U0010ffff"} {
		class, _, err := charClass([]rune("["+body+"]"), 0)
		if err != nil {
			t.Fatal(err)
		}
		re, err := regexp.Compile("^(?:" + class + ")$")
		if err != nil || re.MatchString("/") {
			t.Fatalf("class %q crosses a directory boundary: %q, %v", body, class, err)
		}
	}
	parsed, err := Parse("dir[.-0]file.log 1d", "test")
	if err != nil {
		t.Fatal(err)
	}
	set := Set{Rules: parsed}
	if _, ok := set.decide("dir/file.log", false); ok {
		t.Fatal("a character class registered a path across a directory separator")
	}
	for _, path := range []string{"dir.file.log", "dir0file.log"} {
		if _, ok := set.decide(path, false); !ok {
			t.Fatalf("removing the separator changed another range member: %s", path)
		}
	}
}

// Exercise successful parse/encode/parse cycles as well as rejection to check
// round-trip stability. Keep the corpus on disk for ordinary go test.
func FuzzParseRules(f *testing.F) {
	f.Fuzz(func(t *testing.T, text, path string) {
		rules, err := Parse(text, "fuzz")
		if err != nil {
			return
		}
		var encoded strings.Builder
		for _, r := range rules {
			if r.Negate {
				fmt.Fprintf(&encoded, "!%s\n", r.Pattern)
			} else {
				fmt.Fprintf(&encoded, "%s %ds\n", r.Pattern, r.Lifespan/time.Second)
			}
		}
		again, err := Parse(encoded.String(), "fuzz")
		if err != nil || len(again) != len(rules) {
			t.Fatalf("rules round trip: %v, %q", err, encoded.String())
		}
		for i, r := range rules {
			got := again[i]
			if r.Pattern != got.Pattern || r.Negate != got.Negate || r.DirOnly != got.DirOnly || r.Lifespan != got.Lifespan ||
				r.re.MatchString(path) != got.re.MatchString(path) {
				t.Fatalf("rule changed: %+v -> %+v", r, got)
			}
			if r.Negate && r.Lifespan != 0 || !r.Negate && r.Lifespan <= 0 {
				t.Fatalf("invalid lifespan: %+v", r)
			}
		}
	})
}

func FuzzRulePattern(f *testing.F) {
	f.Fuzz(func(t *testing.T, pattern, path string) {
		if re, err := compile(pattern); err == nil {
			encoded, err := re.MarshalText()
			if err != nil {
				t.Fatal(err)
			}
			var again regexp.Regexp
			if err := again.UnmarshalText(encoded); err != nil || re.MatchString(path) != again.MatchString(path) {
				t.Fatalf("compiled pattern changed on round trip: %q, %v", pattern, err)
			}
		}
		// Feed the same bytes directly to the bracket translator, so malformed
		// outer rules do not prevent exploration of escapes and range endpoints.
		runes := []rune("[" + pattern + "]")
		class, end, err := charClass(runes, 0)
		if err != nil {
			return
		}
		if end <= 0 || end >= len(runes) || runes[end] != ']' {
			t.Fatalf("invalid closing bracket index: %d in %q", end, string(runes))
		}
		re, err := regexp.Compile("^(?:" + class + ")$")
		if err == nil && (re.MatchString("/") || re.MatchString("") || re.MatchString("aa")) {
			t.Fatalf("character class must match one non-separator: %q -> %q", string(runes[:end+1]), class)
		}
	})
}
