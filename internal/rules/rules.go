// Package rules decides which paths agents leave behind get a lifespan.
//
// A rules file uses .gitignore patterns followed by a lifespan:
//
//	# pattern            lifespan
//	output/**            7d
//	*.log                1d
//	.playwright-mcp/     1d
//	test-results/        off      # disable a built-in rule
//	!output/keep/**               # never give these a lifespan
//
// Built-in rules come first, then the project's .obitrules (only once the
// user trusts it); the last matching rule wins. A directory rule makes the
// whole directory the unit that gets a lifespan, so one artefact folder is one
// entry rather than thousands.
package rules

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// FileName is the project rules file, kept at the project root.
const FileName = ".obitrules"

// Builtin are conservative rules for artefacts AI agents commonly leave.
const Builtin = `# Built-in rules: artefacts AI agents commonly leave behind.
.playwright-mcp/      1d
.playwright-cli/      1d
test-results/         3d
playwright-report/    3d
__pycache__/          7d
.pytest_cache/        7d
.mypy_cache/          7d
.ruff_cache/          7d
*.log                 3d
*.tmp                 1d
`

// Rule is one parsed line.
type Rule struct {
	Source   string        `json:"source"` // "builtin" or the rules file path
	Line     int           `json:"line"`
	Pattern  string        `json:"pattern"`
	Negate   bool          `json:"negate,omitempty"`   // !pattern or "off": never give a lifespan
	DirOnly  bool          `json:"dirOnly,omitempty"`  // pattern ended in /
	Lifespan time.Duration `json:"lifespan,omitempty"` // zero when negated
	re       *regexp.Regexp
}

// Set is an ordered list of rules relative to one base directory.
type Set struct {
	Base  string `json:"base"`
	Rules []Rule `json:"rules"`
}

// Match is the rule decision for one path.
type Match struct {
	Unit     string        `json:"unit"` // the path that gets the lifespan (the path itself or a matching ancestor)
	Lifespan time.Duration `json:"lifespan"`
	Rule     Rule          `json:"rule"`
}

// Match finds the unit and lifespan for path (absolute, inside the base).
// Each ancestor directory is tried from the base downwards; the first one a
// rule gives a lifespan to becomes the unit, as with ignored directories in
// .gitignore. isDir says whether path itself is a directory. A failed scan
// returns an error, never permission to register a smaller unit.
func (s Set) Match(path string, isDir bool) (Match, bool, error) {
	rel, err := filepath.Rel(s.Base, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return Match{}, false, nil
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for k := 1; k <= len(parts); k++ {
		prefix := strings.Join(parts[:k], "/")
		dir := k < len(parts) || isDir
		rule, ok := s.decide(prefix, dir)
		if ok && rule.Negate {
			// Protection is inherited: a negated directory covers everything
			// in it, whatever positive rule would match further down.
			return Match{}, false, nil
		}
		if !ok {
			continue
		}
		if dir {
			collapse, err := s.collapsible(prefix)
			if err != nil {
				return Match{}, false, err
			}
			if !collapse {
				continue // something inside is protected: look for a smaller unit
			}
		}
		return Match{Unit: filepath.Join(s.Base, filepath.FromSlash(prefix)), Lifespan: rule.Lifespan, Rule: rule}, true, nil
	}
	return Match{}, false, nil
}

// Protects reports whether a negation covers path or one of its ancestors,
// or, for a directory, holds (or could later hold) something protected.
// Scan errors leave the decision unknown and must be retried.
func (s Set) Protects(path string, isDir bool) (bool, error) {
	rel, err := filepath.Rel(s.Base, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return false, nil
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for k := 1; k <= len(parts); k++ {
		dir := k < len(parts) || isDir
		if r, ok := s.decide(strings.Join(parts[:k], "/"), dir); ok && r.Negate {
			return true, nil
		}
	}
	if !isDir {
		return false, nil
	}
	collapse, err := s.collapsible(filepath.ToSlash(rel))
	return !collapse, err
}

// walkLimit bounds the scan that checks a directory unit for protected entries.
var walkLimit = 20_000

// collapsible reports whether the directory prefix may become one unit:
// negation always wins, so a directory that holds, or could later hold, a
// path a negated rule protects is not collapsed. Unlike .gitignore, where an
// excluded parent cannot be re-included, protection here reaches through.
func (s Set) collapsible(prefix string) (bool, error) {
	var negations []Rule
	for _, r := range s.Rules {
		if r.Negate {
			negations = append(negations, r)
		}
	}
	if len(negations) == 0 {
		return true, nil
	}
	scope := prefix + "/"
	for _, r := range negations {
		if lit := literalPrefix(r.Pattern); strings.Contains(strings.TrimSuffix(strings.TrimPrefix(r.Pattern, "/"), "/"), "/") &&
			(hasPrefixFold(lit, scope) || hasPrefixFold(scope, lit)) {
			return false, nil // an anchored negation reaches into this directory
		}
	}
	dir := filepath.Join(s.Base, filepath.FromSlash(prefix))
	if _, err := os.Lstat(dir); errors.Is(err, os.ErrNotExist) {
		return true, nil // nothing on disk yet that a negation could protect
	} else if err != nil {
		return false, err
	}
	collapsible, seen := true, 0
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if seen++; seen > walkLimit {
			collapsible = false
			return fs.SkipAll
		}
		if p == dir {
			return nil
		}
		rel, err := filepath.Rel(s.Base, p)
		if err != nil {
			return err
		}
		if r, ok := s.decide(filepath.ToSlash(rel), d.IsDir()); ok && r.Negate {
			collapsible = false
			return fs.SkipAll
		}
		return nil
	})
	return collapsible, err
}

// literalPrefix is the part of an anchored pattern before its first wildcard.
func literalPrefix(pattern string) string {
	pattern = strings.TrimPrefix(pattern, "/")
	if i := strings.IndexAny(pattern, "*?[\\"); i >= 0 {
		return pattern[:i]
	}
	return strings.TrimSuffix(pattern, "/") + "/"
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

// decide returns the last rule matching rel, if any.
// decide returns the rule for rel. Unlike .gitignore's last-match-wins, a
// negation that matches wins regardless of order: for a tool that deletes,
// "never touch this" must not be overridden by a later broad pattern.
// Otherwise the last matching positive rule applies.
func (s Set) decide(rel string, isDir bool) (Rule, bool) {
	var found Rule
	ok := false
	for _, r := range s.Rules {
		if r.DirOnly && !isDir {
			continue
		}
		if !r.re.MatchString(rel) {
			continue
		}
		if r.Negate {
			return r, true
		}
		found, ok = r, true
	}
	return found, ok
}

// Load builds the rule set for a project: built-ins, then .obitrules if trusted.
// It reports the project file's trust state so callers can explain skips.
func Load(projectRoot string, trust TrustStore) (Set, FileState, error) {
	builtin, err := Parse(Builtin, "builtin")
	if err != nil {
		return Set{}, FileState{}, err
	}
	set := Set{Base: projectRoot, Rules: builtin}
	state, err := trust.Check(filepath.Join(projectRoot, FileName))
	if err != nil {
		return set, state, err
	}
	if state.Status == Absent {
		return set, state, nil
	}
	if state.Status == Missing {
		return Set{}, state, fmt.Errorf("previously trusted .obitrules is missing; automatic registration paused until obit untrust or reviewed rules are restored and trusted")
	}
	project, err := Parse(state.content, state.Path)
	if err != nil {
		return Set{}, state, fmt.Errorf("%w; automatic registration paused; fix .obitrules, then run obit trust", err)
	}
	if state.Status == Changed {
		return Set{}, state, fmt.Errorf(".obitrules changed; automatic registration paused until obit trust")
	}
	if state.Status != Trusted && state.Status != EnvOverride {
		return set, state, nil
	}
	set.Rules = append(set.Rules, project...)
	return set, state, nil
}

func readFile(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return string(data), true, nil
}
