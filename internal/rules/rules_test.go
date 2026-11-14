package rules

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLoadPausesInvalidOrChangedProjectRules(t *testing.T) {
	root := t.TempDir()
	trust := TrustStore{Path: filepath.Join(t.TempDir(), "trusted_rules.json")}
	path := filepath.Join(root, FileName)
	for _, content := range []string{"[broken 2h\n", "*.log 2h\n"} {
		if err := os.WriteFile(path, []byte("*.log 1d\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := trust.Trust(path, ""); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		set, _, err := Load(root, trust)
		if err == nil || !strings.Contains(err.Error(), "automatic registration paused") || !strings.Contains(err.Error(), "obit trust") || len(set.Rules) != 0 {
			t.Fatalf("project rule failure allowed builtins: %+v %v", set, err)
		}
		if strings.HasPrefix(content, "[") && !strings.Contains(err.Error(), ".obitrules:1:") {
			t.Fatalf("missing invalid rule line: %v", err)
		}
	}
}

func TestMatchPicksTheUnitAndLifespan(t *testing.T) {
	t.Parallel()
	base := testenv.NativeFixture(`C:\repo`)
	s := set(t, base, `
output/**          7d
!output/keep/**
test-results/      off
/build/            2d
*.png              1h   # agent screenshots
`)
	cases := []struct {
		path  string
		isDir bool
		unit  string
		life  time.Duration
	}{
		{testenv.NativeFixture(`C:\repo\logs\app.log`), false, testenv.NativeFixture(`C:\repo\logs\app.log`), 72 * time.Hour},                // builtin *.log at any depth
		{testenv.NativeFixture(`C:\repo\.playwright-mcp\shot.png`), false, testenv.NativeFixture(`C:\repo\.playwright-mcp`), 24 * time.Hour}, // directory rule wins at the top
		{testenv.NativeFixture(`C:\repo\output\run1\a.txt`), false, testenv.NativeFixture(`C:\repo\output\run1`), 7 * 24 * time.Hour},        // output/** names each child, not output
		{testenv.NativeFixture(`C:\repo\build\x.o`), false, testenv.NativeFixture(`C:\repo\build`), 48 * time.Hour},                          // anchored directory
		{testenv.NativeFixture(`C:\repo\shot.PNG`), false, testenv.NativeFixture(`C:\repo\shot.PNG`), time.Hour},                             // case-insensitive on Windows and macOS
	}
	for _, c := range cases {
		if runtime.GOOS != "windows" && runtime.GOOS != "darwin" && strings.HasSuffix(c.path, ".PNG") {
			if _, ok, err := s.Match(c.path, c.isDir); err != nil || ok {
				t.Fatalf("matching must preserve case on %s: %v %v", runtime.GOOS, ok, err)
			}
			continue
		}
		m, ok, matchErr := s.Match(c.path, c.isDir)
		if matchErr != nil || !ok || m.Unit != c.unit || m.Lifespan != c.life {
			t.Errorf("Match(%s) = %+v, %v; want unit %s lifespan %v", c.path, m, ok, c.unit, c.life)
		}
	}
	for _, path := range []string{
		testenv.NativeFixture(`C:\repo\output\keep\a.txt`),       // negated
		testenv.NativeFixture(`C:\repo\test-results\report.xml`), // builtin turned off
		testenv.NativeFixture(`C:\repo\src\build\x.o`),           // /build/ is anchored to the root
		testenv.NativeFixture(`C:\repo\src\main.go`),             // no rule
		testenv.NativeFixture(`C:\elsewhere\a.log`),              // outside the base
	} {
		if m, ok, matchErr := s.Match(path, false); matchErr != nil || ok {
			t.Errorf("Match(%s) = %+v; want no lifespan", path, m)
		}
	}
	if _, ok, matchErr := s.Match(testenv.NativeFixture(`C:\repo\output`), true); matchErr != nil || ok {
		t.Error("output/** must not give output itself a lifespan")
	}
}

func TestParseRejectsMalformedLines(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"output/**", "*.log 3x", "!keep 1d", "a b c"} {
		if _, err := Parse(text, "x"); err == nil {
			t.Errorf("Parse(%q) succeeded; want an error", text)
		}
	}
}

func TestProjectRulesLoadOnlyOnceTrusted(t *testing.T) {
	root := t.TempDir()
	trust := TrustStore{Path: filepath.Join(t.TempDir(), "trusted_rules.json")}
	file := filepath.Join(root, FileName)
	if err := os.WriteFile(file, []byte("** 1m\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(root, "notes.txt")

	s, state, err := Load(root, trust)
	if _, ok, matchErr := s.Match(victim, false); matchErr != nil || err != nil || state.Status != Untrusted || ok {
		t.Fatalf("untrusted rules must be skipped: %v %s %v", err, state.Status, ok)
	}
	if _, err := trust.Trust(file, ""); err != nil {
		t.Fatal(err)
	}
	if s, state, _ = Load(root, trust); state.Status != Trusted {
		t.Fatalf("want trusted, got %s", state.Status)
	}
	if _, ok, matchErr := s.Match(victim, false); matchErr != nil || !ok {
		t.Fatal("trusted rules must apply")
	}
	if err := os.WriteFile(file, []byte("** 1m\n*.txt 1m\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if s, state, _ = Load(root, trust); state.Status != Changed {
		t.Fatalf("an edited file must need a new review, got %s", state.Status)
	}
	if _, ok, matchErr := s.Match(victim, false); matchErr != nil || ok {
		t.Fatal("changed rules must be skipped")
	}
	t.Setenv("OBIT_TRUST_PROJECT_RULES", "1")
	if _, state, _ = Load(root, trust); state.Status != EnvOverride {
		t.Fatalf("env override not honoured, got %s", state.Status)
	}
}

func TestDirectoryHoldingAProtectedFileIsNotCollapsed(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	for _, f := range []string{`output/run1/a.txt`, `output/run1/notes.keep`, `output/run2/b.txt`} {
		path := filepath.Join(base, f)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s := set(t, base, "output/** 7d\n!*.keep\n")

	if m, ok, matchErr := s.Match(filepath.Join(base, `output/run1/a.txt`), false); matchErr != nil || !ok || m.Unit != filepath.Join(base, `output/run1/a.txt`) {
		t.Fatalf("run1 holds a protected file, so only a.txt may be the unit; got %+v %v", m, ok)
	}
	if m, ok, matchErr := s.Match(filepath.Join(base, `output/run2/b.txt`), false); matchErr != nil || !ok || m.Unit != filepath.Join(base, `output/run2`) {
		t.Fatalf("run2 holds nothing protected and should be one unit; got %+v %v", m, ok)
	}
	if _, ok, matchErr := s.Match(filepath.Join(base, `output/run1/notes.keep`), false); matchErr != nil || ok {
		t.Fatal("the protected file itself must never get a lifespan")
	}
}

func TestNegationWinsWhateverTheOrderAndCoversDescendants(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	s := set(t, base, "!keep/\n!keep.log\n*.log 1d\n")
	for _, rel := range []string{`keep/a.log`, `keep/deep/b.log`, `keep.log`} {
		if m, ok, matchErr := s.Match(filepath.Join(base, rel), false); matchErr != nil || ok {
			t.Errorf("%s is protected by a negation, got %+v", rel, m)
		}
	}
	if _, ok, matchErr := s.Match(filepath.Join(base, "other.log"), false); matchErr != nil || !ok {
		t.Error("other.log is not protected and should match *.log")
	}
}

func TestNonASCIIAndCharacterClassesWork(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	s := set(t, base, "!重要.log\n!report[0-9].log\n!draft[!a-z].txt\n*.txt 1d\n")
	for _, rel := range []string{"重要.log", "report1.log", "draft7.txt"} {
		if m, ok, matchErr := s.Match(filepath.Join(base, rel), false); matchErr != nil || ok {
			t.Errorf("%s must be protected, got %+v", rel, m)
		}
	}
	for _, rel := range []string{"不重要.log", "reportx.log", "drafta.txt"} {
		if _, ok, matchErr := s.Match(filepath.Join(base, rel), false); matchErr != nil || !ok {
			t.Errorf("%s is not protected and should match a positive rule", rel)
		}
	}
	if _, err := Parse("report[0-9.log 1d\n", "x"); err == nil {
		t.Error("an unclosed [ must be rejected, not treated literally")
	}
}

func TestEscapedCharacterClassIsLiteral(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	s := set(t, base, "report[a\\-z].dat 1d\n!report[a\\-z].log\n")
	for _, name := range []string{"reporta.dat", "report-.dat", "reportz.dat"} {
		if _, ok, matchErr := s.Match(filepath.Join(base, name), false); matchErr != nil || !ok {
			t.Errorf("literal class must match %s", name)
		}
	}
	if _, ok, matchErr := s.Match(filepath.Join(base, "reportb.dat"), false); matchErr != nil || ok {
		t.Error("escaped hyphen must not create a range")
	}
	if _, ok, matchErr := s.Match(filepath.Join(base, "report-.log"), false); matchErr != nil || ok {
		t.Error("negated literal class must protect report-.log from builtins")
	}
}

func TestEscapedNonASCIICharacterClass(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	s := set(t, base, "[\\é\\中].dat 1d\n![\\é\\中].log\n")
	for _, name := range []string{"é", "中"} {
		if _, ok, matchErr := s.Match(filepath.Join(base, name+".dat"), false); matchErr != nil || !ok {
			t.Errorf("escaped non-ASCII must match %s", name)
		}
		if _, ok, matchErr := s.Match(filepath.Join(base, name+".log"), false); matchErr != nil || ok {
			t.Errorf("escaped non-ASCII must protect %s", name)
		}
	}
}

func TestTrustRefusesContentChangedAfterReview(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	trust := TrustStore{Path: filepath.Join(t.TempDir(), "trusted_rules.json")}
	file := filepath.Join(root, FileName)
	if err := os.WriteFile(file, []byte("*.log 1d\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	shown, err := trust.Check(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("** 1m\n"), 0o600); err != nil { // swapped while the user reads
		t.Fatal(err)
	}
	if _, err := trust.Trust(file, shown.SHA256); err == nil {
		t.Fatal("content changed after review must not be trusted")
	}
	if state, _ := trust.Check(file); state.Status != Untrusted {
		t.Fatalf("got %s", state.Status)
	}
}

// Editing backups get a week as a way back; names that merely contain "bak"
// or "old" are not backups.
func TestBuiltinRulesCoverBackups(t *testing.T) {
	t.Parallel()
	base := testenv.NativeFixture(`C:\repo`)
	s := set(t, base, "")
	for _, name := range []string{"config.json.bak", "settings.json.bak.1", "app.yaml.bak-20261114", "a.toml.bak_old", "main.go.orig", "patch.rej", "notes.old", "db.backup", "report.txt~"} {
		path := filepath.Join(base, "src", name)
		if m, ok, err := s.Match(path, false); err != nil || !ok || m.Lifespan != 7*24*time.Hour {
			t.Errorf("Match(%s) = %+v %v %v; want a 7d backup lifespan", name, m, ok, err)
		}
	}
	for _, name := range []string{"foo.bakery.txt", "bakfile.txt", "old.txt", "golden.oldest", "backup.go", "orig.md", "LOG.old", "log.old"} {
		if m, ok, err := s.Match(filepath.Join(base, "src", name), false); err != nil || ok {
			t.Errorf("Match(%s) = %+v; not a backup", name, m)
		}
	}
}
