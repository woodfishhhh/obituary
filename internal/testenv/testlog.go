package testenv

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"go/version"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var errUnknownTestLog = errors.New("unrecognized Go test log protocol")

var explainUnchangedTestLog = sync.OnceFunc(func() {
	fmt.Fprintln(os.Stderr, "test cache log: kept unchanged; Go version or log format is outside the verified Go 1.26 protocol")
})

// Go 1.26 testing.M.after closes this per-run log before M.Run returns.
// cmd/go/internal/test/test.go:computeTestInputsID rechecks every entry; Windows
// path expansion can record millions of identical ancestors. Keep every cache
// dependency, but check repeated absolute names and environment keys once.
// Relative accesses and all chdir entries retain their original order/context.
// Use the per-run file because testing exposes no public logger decorator.
// Go's dependency checks and test cache remain enabled.
// After Main successfully removes its scratch, absolute accesses under that
// deleted, module-external root cannot resolve back into the module. Go's
// search.InDir ignores them, but otherwise re-resolves every missing name.
func compactTestLog(scratch string) error {
	path := flag.Lookup("test.testlogfile").Value.String()
	if path == "" {
		return nil
	}
	return compactTestLogFile(path, scratch, runtime.Version())
}

func compactTestLogFile(path, scratch, goVersion string) error {
	// go/version also accepts custom toolchain suffixes. Only the stable,
	// unmodified 1.26.x family was checked against testing/internal/testdeps.
	if !version.IsValid(goVersion) || !strings.HasPrefix(goVersion, "go1.26.") || strings.Contains(goVersion, "-") {
		explainUnchangedTestLog()
		return nil
	}
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.CreateTemp(filepath.Dir(path), "testlog-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(out.Name())
	err = uniqueTestInputs(in, out, scratch)
	closeErr := errors.Join(in.Close(), out.Close())
	if errors.Is(err, errUnknownTestLog) {
		explainUnchangedTestLog()
		return closeErr
	}
	err = errors.Join(err, closeErr)
	if err != nil {
		return err
	}
	return os.Rename(out.Name(), path)
}

func uniqueTestInputs(input io.Reader, output io.Writer, scratch string) error {
	in, out := bufio.NewReader(input), bufio.NewWriter(output)
	header, err := in.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if header != "# test log\n" {
		return errUnknownTestLog
	}
	if _, err := out.WriteString(header); err != nil {
		return err
	}
	seen := make(map[string]bool)
	for {
		line, err := in.ReadString('\n')
		if errors.Is(err, io.EOF) && line == "" {
			return out.Flush()
		}
		if errors.Is(err, io.EOF) {
			return errUnknownTestLog // a partial record was not emitted by StopTestLog
		}
		if err != nil {
			return err
		}
		op, name, _ := strings.Cut(strings.TrimSuffix(line, "\n"), " ")
		if name == "" || (op != "getenv" && op != "stat" && op != "open" && op != "chdir") {
			return errUnknownTestLog
		}
		if op == "chdir" && !filepath.IsAbs(name) {
			return errUnknownTestLog // cmd/go treats the recorded cwd as absolute
		}
		if (op == "stat" || op == "open") && insideScratch(scratch, name) {
			continue
		}
		independent := op == "getenv" || (op == "stat" || op == "open") && filepath.IsAbs(name)
		if !independent || !seen[line] {
			if _, err := out.WriteString(line); err != nil {
				return err
			}
			if independent {
				seen[line] = true
			}
		}
	}
}

func insideScratch(scratch, path string) bool {
	if scratch == "" || !filepath.IsAbs(path) {
		return false
	}
	// Keep aliases and alternate spellings; only the root Main actually
	// removed is eligible, and Rel still rejects an escaping .. component.
	if path != scratch && !strings.HasPrefix(path, scratch+string(filepath.Separator)) {
		return false
	}
	rel, err := filepath.Rel(scratch, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Unknown module metadata keeps all dependencies.
// In particular, a caller-provided TEMP inside its module must not be filtered.
func scratchOutsideModule(scratch string) bool {
	dir, err := os.Getwd()
	if err != nil {
		return false
	}
	for {
		_, err := os.Stat(filepath.Join(dir, "go.mod"))
		if err == nil {
			root, err := filepath.EvalSymlinks(dir)
			if err != nil {
				return false
			}
			rel, err := filepath.Rel(root, scratch)
			// Both paths are absolute and resolved. Different volumes cannot
			// have the module prefix either (the same rule used by search.InDir).
			return err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
		}
		if !os.IsNotExist(err) || filepath.Dir(dir) == dir {
			return false
		}
		dir = filepath.Dir(dir)
	}
}
