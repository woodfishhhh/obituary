package rules

import (
	"bufio"
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/woodfishhhh/obituary/internal/core/lifespan"
)

// Parse reads rules text. source labels the rules in diagnostics.
func Parse(text, source string) ([]Rule, error) {
	if !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		return nil, &ParseError{Source: source, Line: 1, Kind: "encoding", Cause: errors.New("rules must be UTF-8 (UTF-8 BOM is accepted)")}
	}
	text = strings.TrimPrefix(text, "\ufeff")
	var out []Rule
	scanner := bufio.NewScanner(strings.NewReader(text))
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimSpace(scanner.Text())
		if i := strings.Index(line, " #"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		r, err := parseLine(line, source, n)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := scanner.Err(); err != nil {
		return nil, &ParseError{Source: source, Kind: "read", Cause: err}
	}
	return out, nil
}

func parseLine(line, source string, n int) (Rule, error) {
	fields := strings.Fields(line)
	r := Rule{Source: source, Line: n}
	fail := func(kind string, err error) (Rule, error) {
		return r, &ParseError{Source: source, Line: n, Kind: kind, Cause: err}
	}
	pattern := fields[0]
	negated := strings.HasPrefix(pattern, "!")
	if negated && len(fields) > 1 {
		return fail("syntax", errors.New("a negated pattern takes no lifespan"))
	}
	if !negated && len(fields) != 2 {
		return fail("syntax", errors.New("want `pattern lifespan` (for example `output/** 7d`)"))
	}
	if negated {
		r.Negate, pattern = true, pattern[1:]
	} else {
		d, err := ruleLifespan(fields[1])
		if err != nil {
			return fail("lifespan", err)
		}
		r.Lifespan, r.Negate = d, d == 0
	}
	if pattern == "" || pattern == "/" {
		return fail("pattern", errors.New("empty pattern"))
	}
	r.DirOnly, r.Pattern = strings.HasSuffix(pattern, "/"), pattern
	re, err := compile(strings.TrimSuffix(pattern, "/"))
	if err != nil {
		return fail("pattern", err)
	}
	r.re = re
	return r, nil
}

func ruleLifespan(text string) (time.Duration, error) {
	if strings.EqualFold(text, "off") || strings.EqualFold(text, "never") {
		return 0, nil
	}
	return lifespan.ParseDuration(text)
}

// compile turns a .gitignore pattern into a regexp over slash-separated
// paths relative to the rules base. A pattern without an inner slash matches
// at any depth; one with a slash is anchored to the base.
//
// Keep this compiler rather than go-git v5.19.3's gitignore.Pattern. That
// library treats a malformed pattern as "no match", so a broken negation
// would silently stop protecting. It matches with filepath.Match, which is
// case-sensitive, has no [!...] negation and no backslash escapes on
// Windows. It does not backtrack across "**", and it lets dir/** match dir
// itself, which would widen registration units. Keeping our behaviour would
// still need this matcher.
func compile(pattern string) (*regexp.Regexp, error) {
	anchored := strings.Contains(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")
	var b strings.Builder
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		b.WriteString("(?i)") // case-insensitive file systems
	}
	if anchored {
		b.WriteString("^")
	} else {
		b.WriteString("^(?:.*/)?")
	}
	// Work on runes, not bytes, so non-ASCII names (中文.log) stay intact.
	runes := []rune(pattern)
	has := func(i int, s string) bool { return strings.HasPrefix(string(runes[i:]), s) }
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch {
		case has(i, "**/"):
			b.WriteString("(?:.*/)?")
			i += 2
		case has(i, "/**") && i+3 == len(runes):
			b.WriteString("/.+") // everything inside, not the directory itself
			i += 2
		case has(i, "**"):
			b.WriteString(".*")
			i++
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		case c == '[':
			class, next, err := charClass(runes, i)
			if err != nil {
				return nil, err
			}
			b.WriteString(class)
			i = next
		case c == '\\' && i+1 < len(runes):
			i++
			b.WriteString(regexp.QuoteMeta(string(runes[i])))
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

// charClass translates a .gitignore bracket expression starting at runes[i]
// ("[0-9]", "[!a-z]", "[]x]") and returns it with the index of its closing
// bracket. Like Git, a class never matches "/". An unclosed bracket is an
// error rather than a silently literal pattern, because a protection rule
// that quietly matches nothing is worse than a rejected file.
func charClass(runes []rune, i int) (string, int, error) {
	j := i + 1
	negate := j < len(runes) && (runes[j] == '!' || runes[j] == '^')
	if negate {
		j++
	}
	var body strings.Builder
	first := true
	for ; j < len(runes); j++ {
		c := runes[j]
		if c == ']' && !first {
			if negate {
				return "[^/" + body.String() + "]", j, nil
			}
			class, err := classWithoutSeparator("[" + body.String() + "]")
			return class, j, err
		}
		first = false
		switch {
		case c == '\\' && j+1 < len(runes):
			j++
			c = runes[j]
			if c == '/' {
				continue // classes never match a separator, even escaped
			}
			if c < 128 && ((c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9')) {
				body.WriteRune('\\')
			}
			body.WriteRune(c)
		case c == '-':
			body.WriteRune('-')
		case c == '/':
			// never matches a separator
		default:
			body.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	return "", 0, fmt.Errorf("unclosed [ in pattern %q", string(runes))
}

// Use Go's regexp parser to expand ranges: removing literal slashes alone
// still lets [.-0] match '/'. RE2 has no class intersection or lookahead, so
// subtract the separator from the parsed rune ranges before compiling.
func classWithoutSeparator(class string) (string, error) {
	re, err := syntax.Parse(class, syntax.Perl)
	if err != nil {
		return "", err
	}
	var ranges []rune
	switch re.Op {
	case syntax.OpCharClass:
		ranges = re.Rune
	case syntax.OpAnyChar:
		ranges = []rune{0, utf8.MaxRune}
	case syntax.OpAnyCharNotNL:
		ranges = []rune{0, '\n' - 1, '\n' + 1, utf8.MaxRune}
	default:
		return re.String(), nil // a literal or an empty set cannot contain '/'
	}
	filtered := &syntax.Regexp{Op: syntax.OpCharClass}
	for i := 0; i < len(ranges); i += 2 {
		lo, hi := ranges[i], ranges[i+1]
		if lo <= '/' && '/' <= hi {
			if lo < '/' {
				filtered.Rune = append(filtered.Rune, lo, '/'-1)
			}
			if hi > '/' {
				filtered.Rune = append(filtered.Rune, '/'+1, hi)
			}
		} else {
			filtered.Rune = append(filtered.Rune, lo, hi)
		}
	}
	return filtered.String(), nil
}
