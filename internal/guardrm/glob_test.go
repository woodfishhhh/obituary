package guardrm

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLeadingGlobChecksReturnedEntriesBeforeBudget(t *testing.T) {
	t.Parallel()
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "-i"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	dash, err := os.Stat(filepath.Join(cwd, "-i"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := os.Stat(cwd)
	if err != nil {
		t.Fatal(err)
	}
	for _, budget := range []string{"time", "count"} {
		t.Run(budget, func(t *testing.T) {
			read := 0
			why := scanLeadingGlob(func(n int) ([]os.DirEntry, error) {
				entries := make([]os.DirEntry, n)
				for i := range entries {
					entries[i] = fs.FileInfoToDirEntry(plain)
				}
				read += n
				if budget == "time" || read == 5001 {
					entries[len(entries)-1] = fs.FileInfoToDirEntry(dash)
					return entries, io.EOF
				}
				return entries, nil
			}, func() time.Duration {
				if budget == "time" && read > 0 {
					return 50 * time.Millisecond
				}
				return 0
			})
			if why != "leading glob before -- may expand into options" {
				t.Fatalf("known option-shaped entry lost at %s budget: %q", budget, why)
			}
		})
	}
}

func TestSuggestLeadingGlobOptions(t *testing.T) {
	t.Parallel()
	for _, prefix := range []string{"rm ", "rmdir ", "git clean -fdx "} {
		for _, ref := range []string{"*", "*.log", "?n", "[-a]*"} {
			command := prefix + ref
			if got, why := Suggest(command, Context{Shell: Bash}); got != command || why == "" {
				t.Errorf("possible option glob suggested: %q (%s)", got, why)
			}
		}
		for _, ref := range []string{"build/*", "-- *", "-- *.log"} {
			command := prefix + ref
			want := "obit " + strings.Replace(command, "git clean", "git-clean", 1)
			if got, why := Suggest(command, Context{Shell: Bash}); got != want || why != "" {
				t.Errorf("safe glob refused: %q (%s)", got, why)
			}
		}
	}
}

func BenchmarkLeadingGlobReadDir(b *testing.B) {
	for _, count := range []int{0, 100, 1000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			cwd := b.TempDir()
			for i := range count {
				if err := os.WriteFile(filepath.Join(cwd, fmt.Sprintf("file-%04d.log", i)), nil, 0o600); err != nil {
					b.Fatal(err)
				}
			}
			b.Run("ReadDir", func(b *testing.B) {
				for b.Loop() {
					if entries, err := os.ReadDir(cwd); err != nil || len(entries) != count {
						b.Fatalf("ReadDir: %d entries, %v", len(entries), err)
					}
				}
			})
			for _, command := range []string{"rm -f *.log", "rm -f *.log *.tmp"} {
				b.Run(command, func(b *testing.B) {
					for b.Loop() {
						if got, why := Suggest(command, Context{Shell: Bash, Cwd: cwd}); got != "obit "+command || why != "" {
							b.Fatalf("glob suggestion: %q (%s)", got, why)
						}
					}
				})
			}
		})
	}
}

func TestLeadingGlobChecksAfterSyntax(t *testing.T) {
	t.Parallel()
	// A premature scan would report option ambiguity instead of the syntax reason.
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "-n"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ command, reason string }{
		{"git clean -ndx *", "no supported deletion command"},
		{"rm --unknown *", "unsupported option"},
		{"rm * -f", "options after operands"},
		{"rm * keep.", "trailing dot or space"},
		{"rm *; echo done", "final command segment"},
		{"echo ready; rm *", "only literal cd"},
		{"rm * $paths", "dynamic argument"},
	} {
		t.Run(tc.command, func(t *testing.T) {
			if got, why := Suggest(tc.command, Context{Shell: Bash, Cwd: cwd}); got != tc.command || !strings.Contains(why, tc.reason) {
				t.Fatalf("directory inspection preceded syntax decision: %q (%s), want %s", got, why, tc.reason)
			}
		})
	}
}

func TestLeadingGlobLargeDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("enumerates thousands of real directory entries")
	}
	t.Parallel()
	cwd := t.TempDir()
	for i := range 5001 {
		if err := os.WriteFile(filepath.Join(cwd, fmt.Sprintf("file-%05d.log", i)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	started := time.Now()
	command := "rm -f *.log"
	got, why := Suggest(command, Context{Shell: Bash, Cwd: cwd})
	t.Logf("5001 entries: Suggest took %s (%s)", time.Since(started), why)
	if got != "obit "+command || why != "" {
		t.Fatalf("oversized directory must still deny: %q (%s)", got, why)
	}
}

func TestLeadingGlobScanBudgets(t *testing.T) {
	t.Parallel()
	info, err := os.Stat(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	entry := fs.FileInfoToDirEntry(info)
	for _, tc := range []struct {
		name           string
		count, delayAt int
		wantReads      int
	}{
		{"empty", 0, -1, 1},
		{"at-limit", 5000, -1, 41},
		{"over-limit", 5001, -1, 40},
		{"slow-open", 1, 0, 0},
		{"slow-read", 1, 1, 1},
		{"slow-next-batch", 256, 2, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads, remaining := 0, tc.count
			why := scanLeadingGlob(func(n int) ([]os.DirEntry, error) {
				reads++
				if n < 1 || n > 128 {
					t.Fatalf("unbounded directory read: %d", n)
				}
				if remaining == 0 {
					return nil, io.EOF
				}
				entries := make([]os.DirEntry, min(n, remaining))
				for i := range entries {
					entries[i] = entry
				}
				remaining -= len(entries)
				return entries, nil
			}, func() time.Duration {
				if tc.delayAt >= 0 && reads >= tc.delayAt {
					return 50 * time.Millisecond
				}
				return 0
			})
			if why != "" {
				t.Errorf("scan budget must not disable denial: %s", why)
			}
			if reads != tc.wantReads {
				t.Errorf("scan budget: %d reads, want %d", reads, tc.wantReads)
			}
		})
	}
}

func BenchmarkLeadingGlobLargeDirectory(b *testing.B) {
	cwd := b.TempDir()
	for i := range 10000 {
		if err := os.WriteFile(filepath.Join(cwd, fmt.Sprintf("file-%05d.log", i)), nil, 0o600); err != nil {
			b.Fatal(err)
		}
	}
	b.Run("unbounded-ReadDir", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if entries, err := os.ReadDir(cwd); err != nil || len(entries) != 10000 {
				b.Fatalf("ReadDir: %d %v", len(entries), err)
			}
		}
	})
	for _, command := range []string{"rm -f *.log", "git clean -ndx *", "rm --unknown *"} {
		b.Run(command, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				got, why := Suggest(command, Context{Shell: Bash, Cwd: cwd})
				if command == "rm -f *.log" {
					if got != "obit "+command || why != "" {
						b.Fatalf("large directory must still deny: %q (%s)", got, why)
					}
				} else if got != command || why == "" {
					b.Fatalf("non-deletion must pass through: %q (%s)", got, why)
				}
			}
		})
	}
}

func TestLeadingGlobReadFailureStillDenies(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"missing", "not-directory"} {
		t.Run(state, func(t *testing.T) {
			cwd := filepath.Join(t.TempDir(), "unreadable")
			if state == "not-directory" {
				if err := os.WriteFile(cwd, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for _, command := range []string{"rm -f *.log", "git clean -fdx *"} {
				want := "obit " + strings.Replace(command, "git clean", "git-clean", 1)
				if got, why := Suggest(command, Context{Shell: Bash, Cwd: cwd}); got != want || why != "" {
					t.Errorf("read failure revoked denial: %q (%s), want %q", got, why, want)
				}
			}
		})
	}
	if why := scanLeadingGlob(func(int) ([]os.DirEntry, error) {
		return nil, os.ErrPermission
	}, func() time.Duration { return 0 }); why != "" {
		t.Fatalf("ReadDir error revoked denial: %s", why)
	}
}
