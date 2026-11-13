// Package project finds the Git work tree a path belongs to and answers the
// one question automatic registration must never get wrong: does Git track
// this path (or anything inside it)? It asks git itself, so worktrees,
// submodules and sparse checkouts behave as Git sees them. When git cannot
// answer, callers must treat the path as tracked.
package project

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/pathutil"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

var gitTimeout = 15 * time.Second
var gitCommand = exec.CommandContext
var gitOutput = (*exec.Cmd).Output
var gitWithTimeout = context.WithTimeout

// Project is a Git work tree.
type Project struct {
	Root string `json:"root"`
}

// Find returns the work tree containing path, if any. An unreadable discovery
// path is an error, not evidence that the path is outside a project.
func Find(path string) (Project, bool, error) {
	return FindContext(context.Background(), path)
}

// FindContext stops between filesystem probes; an unfinished search must not
// be interpreted as a path outside a repository.
func FindContext(ctx context.Context, path string) (Project, bool, error) {
	if err := ctx.Err(); err != nil {
		return Project{}, false, err
	}
	dir, err := discoveryStart(path)
	if err != nil {
		return Project{}, false, err
	}
	ceilings, err := discoveryCeilings(ctx, dir)
	if err != nil {
		return Project{}, false, err
	}
	// Discover without Git so failures to query tracked paths remain retryable.
	for {
		if err := ctx.Err(); err != nil {
			return Project{}, false, err
		}
		// Each parent becomes an independent Win32 path, including the root
		// returned to Git. A stable full input need not have stable parents.
		if err := pathutil.Unchanged(dir); err != nil {
			return Project{}, false, err
		}
		// Resolve the traversal too (not only the ceilings), including when a
		// missing starting path reaches its first existing ancestor.
		dir, err = discoveryDirectory(dir)
		if err != nil {
			return Project{}, false, err
		}
		for _, ceiling := range ceilings {
			if strings.TrimSuffix(filepath.ToSlash(dir), "/") == ceiling {
				return Project{}, false, nil // the ceiling itself is excluded
			}
		}
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return Project{Root: dir}, true, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return Project{}, false, err
		}
		parent := pathutil.Parent(dir)
		if parent == dir {
			return Project{}, false, nil
		}
		dir = parent
	}
}

func discoveryStart(path string) (string, error) {
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		if _, err := pathutil.Abs(cwd); err != nil {
			return "", err
		}
	}
	dir, err := pathutil.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err != nil || !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	return discoveryDirectory(dir)
}

func discoveryCeilings(ctx context.Context, dir string) ([]string, error) {
	var ceilings []string
	resolve := true
	// Git for Windows v2.54.0 setup.c canonicalize_ceiling_entry and its
	// caller split literally, discard unusable entries, and preserve all bytes
	// after an empty entry. filepath.SplitList instead unquotes Windows PATH.
	for _, ceiling := range strings.Split(os.Getenv("GIT_CEILING_DIRECTORIES"), string(os.PathListSeparator)) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if ceiling == "" {
			resolve = false
			continue
		}
		if !filepath.IsAbs(ceiling) && !strings.HasPrefix(ceiling, `/`) && !strings.HasPrefix(ceiling, `\`) {
			continue
		}
		normalized := ceiling
		if resolve {
			var usable bool
			var err error
			normalized, usable, err = normalizeDiscoveryCeiling(ceiling)
			if err != nil {
				return nil, err
			}
			if !usable {
				continue
			}
		}
		// path.c longest_ancestor_length uses a byte comparison, removes
		// one final slash, and never treats a directory as its own ancestor.
		normalized = strings.TrimSuffix(normalized, "/")
		// A missing descendant may not resolve until traversal reaches an
		// existing junction parent; keep potential ceilings until that point.
		if strings.TrimSuffix(filepath.ToSlash(dir), "/") != normalized {
			ceilings = append(ceilings, normalized)
		}
	}
	return ceilings, nil
}

func normalizeDiscoveryCeiling(ceiling string) (string, bool, error) {
	normalized := ceiling
	if !filepath.IsAbs(ceiling) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", false, err
		}
		cwd, err = pathutil.Abs(cwd)
		if err != nil {
			return "", false, err
		}
		// GetFullPathName under a verbatim cwd can turn a root-relative
		// ceiling into a different UNC name. Use the ordinary volume/share
		// root first, leaving entries after an empty entry byte-for-byte.
		normalized = filepath.Join(pathutil.VolumeRoot(cwd), strings.TrimLeft(ceiling, `\/`))
	}
	normalized, err := discoveryDirectory(normalized)
	if err != nil {
		return "", false, nil
	}
	return filepath.ToSlash(normalized), true, nil
}

// Resolve already expands 8.3 names through GetFinalPathNameByHandle, and
// follows the final junction as Git's real_pathdup does. Reuse that checked
// resolver rather than implementing another Windows alias parser.
func discoveryDirectory(path string) (string, error) {
	abs, err := pathutil.Abs(path)
	if err != nil {
		return "", err
	}
	real, err := winfs.Resolve(abs)
	if errors.Is(err, os.ErrNotExist) {
		return filepath.Clean(abs), nil
	}
	if err != nil {
		return "", &os.PathError{Op: "resolve project directory", Path: abs, Err: err}
	}
	return real, nil
}

func (p Project) git(args ...string) ([]byte, error) {
	return p.gitContext(context.Background(), args...)
}

func (p Project) gitContext(parent context.Context, args ...string) ([]byte, error) {
	ctx, cancel := gitWithTimeout(parent, gitTimeout)
	defer cancel()
	cmd := gitCommand(ctx, "git", append([]string{"-C", p.Root}, args...)...)
	cmd.Env = repositoryEnvironment(cmd.Environ())
	// Bound inherited output pipes too, even if a descendant keeps them open
	// after CommandContext kills Git itself.
	cmd.WaitDelay = time.Second
	if _, bounded := parent.Deadline(); bounded {
		cmd.WaitDelay = 50 * time.Millisecond
	}
	hideWindow(cmd) // the runner has no console; a console git must not open one
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := gitOutput(cmd)
	if ctx.Err() != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), ctx.Err())
	}
	if err != nil {
		var exited *exec.ExitError
		if errors.As(err, &exited) {
			exited.Stderr = stderr.Bytes()
		}
		return nil, fmt.Errorf("git %s: %w %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// Git hooks and agents may inherit another repository's index, work tree or
// configuration. Discovery and queries must describe the same explicit root.
func repositoryEnvironment(inherited []string) []string {
	var env []string
	for _, entry := range inherited {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ToUpper(key)
		switch key {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY",
			"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_COMMON_DIR", "GIT_NAMESPACE", "GIT_PREFIX",
			"GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT", "GIT_SHALLOW_FILE", "GIT_REPLACE_REF_BASE",
			"GIT_NO_REPLACE_OBJECTS", "GIT_GRAFT_FILE", "GIT_LITERAL_PATHSPECS", "GIT_GLOB_PATHSPECS",
			"GIT_NOGLOB_PATHSPECS", "GIT_ICASE_PATHSPECS":
			continue
		}
		if strings.HasPrefix(key, "GIT_CONFIG_KEY_") || strings.HasPrefix(key, "GIT_CONFIG_VALUE_") {
			continue
		}
		env = append(env, entry)
	}
	return env
}

// Rel returns path relative to the project root in Git's slash form.
func (p Project) Rel(path string) (string, error) {
	root, err := pathutil.LongAbs(p.Root)
	if err != nil {
		return "", err
	}
	return (Project{Root: root}).relLongRoot(path)
}

func (p Project) relLongRoot(path string) (string, error) {
	path, err := pathutil.LongAbs(path)
	if err != nil {
		return "", err
	}
	return p.relLongPath(path)
}

func (p Project) relLongPath(path string) (string, error) {
	rel, err := filepath.Rel(p.Root, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("%s is not inside %s", path, p.Root)
	}
	return filepath.ToSlash(rel), nil
}

// Tracked reports whether Git tracks path or any file below it. Pass the
// real path (see FS.RealPath): on case-insensitive volumes the comparison
// also ignores case, since the index may spell a name differently.
func (p Project) Tracked(path string) (bool, error) {
	rel, err := p.Rel(path)
	if err != nil {
		return true, err
	}
	magic := ":(literal)"
	if caseInsensitive {
		magic = ":(literal,icase)"
	}
	out, err := p.git("ls-files", "-z", "--", magic+rel)
	if err != nil {
		return true, err
	}
	return len(out) > 0, nil
}

// TrackedBatch asks Git in bounded batches, with the same literal/icase and
// timeout policy as Tracked. On error every answer is conservatively tracked;
// callers must not interpret the error as evidence that a file is untracked.
func (p Project) TrackedBatch(paths []string) ([]bool, error) {
	return p.TrackedBatchContext(context.Background(), paths)
}

// TrackedBatchContext lets a lock-owning caller impose a shorter shared budget.
func (p Project) TrackedBatchContext(ctx context.Context, paths []string) ([]bool, error) {
	return p.trackedBatchContext(ctx, paths, false)
}

func (p Project) trackedBatchContext(ctx context.Context, paths []string, realNames bool) ([]bool, error) {
	tracked := make([]bool, len(paths))
	for i := range tracked {
		tracked[i] = true
	}
	if len(paths) == 0 {
		return tracked, nil
	}
	if !realNames {
		root, err := pathutil.LongAbs(p.Root)
		if err != nil {
			return tracked, err
		}
		p.Root = root
	}
	batches, rels, err := p.trackedArguments(paths, realNames)
	if err != nil {
		return tracked, err
	}
	var files []string
	for _, batch := range batches {
		out, err := p.gitContext(ctx, batch...)
		if err != nil {
			return tracked, err
		}
		files = append(files, strings.Split(string(out), "\x00")...)
	}
	if caseInsensitive {
		for i := range files {
			files[i] = gitCaseFold(files[i])
		}
	}
	for i, rel := range rels {
		tracked[i] = false
		for _, file := range files {
			if file == rel || strings.HasPrefix(file, rel+"/") {
				tracked[i] = true
				break
			}
		}
	}
	return tracked, nil
}

func (p Project) trackedArguments(paths []string, realNames bool) ([][]string, []string, error) {
	magic := ":(literal)"
	if caseInsensitive {
		magic = ":(literal,icase)"
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return nil, nil, err
	}
	// Split on the rendered command length to stay within the native process
	// limit of 32767 UTF-16 units. Count argv[0], fixed arguments, magic and
	// quoting, with 2048 units spare.
	const commandLimit = 32767 - 2048
	fixed := []string{"ls-files", "-z", "--"}
	baseSize := commandArgSize(gitPath) + commandArgSize("-C") + commandArgSize(p.Root)
	for _, arg := range fixed {
		baseSize += commandArgSize(arg)
	}
	args := append([]string(nil), fixed...)
	size := baseSize
	var batches [][]string
	rels := make([]string, len(paths))
	for i, path := range paths {
		rel, err := p.relLongPath(path)
		if !realNames {
			rel, err = p.relLongRoot(path)
		}
		if err != nil {
			return nil, nil, err
		}
		arg := magic + rel
		argSize := commandArgSize(arg)
		if baseSize+argSize > commandLimit {
			return nil, nil, fmt.Errorf("git pathspec exceeds native command-line limit: %q", path)
		}
		if size+argSize > commandLimit {
			batches = append(batches, args)
			args = append([]string(nil), fixed...)
			size = baseSize
		}
		args = append(args, arg)
		size += argSize
		if caseInsensitive {
			rel = gitCaseFold(rel)
		}
		rels[i] = rel
	}
	return append(batches, args), rels, nil
}

// Git pathspec.h's ps_strncmp uses byte-wise strncasecmp for :(icase),
// not Unicode case folding. See also dir.c and compat/win32/path-utils.c.
func gitCaseFold(path string) string {
	buf := []byte(path)
	for i, b := range buf {
		if b >= 'A' && b <= 'Z' {
			buf[i] = b + ('a' - 'A')
		}
	}
	return string(buf)
}

// Untracked lists untracked paths (ignored ones included), collapsing wholly
// untracked directories into one entry, as `git ls-files --others --directory`.
func (p Project) Untracked() ([]string, error) {
	rels, err := p.others()
	return p.join(rels), err
}

// Ignored lists untracked paths that Git ignores (.gitignore, info/exclude,
// core.excludesFile), collapsing wholly ignored directories into one entry.
// Git also lists a directory that merely holds nothing but ignored files; it
// is dropped (its ignored contents stay listed), because a file written there
// later would not be ignored.
func (p Project) Ignored() ([]string, error) {
	ignored, err := p.others("--ignored", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	plain, err := p.others("--exclude-standard")
	if err != nil {
		return nil, err
	}
	notIgnored := map[string]bool{}
	for _, rel := range plain {
		for dir := rel; dir != "." && dir != "/"; dir = path.Dir(dir) {
			notIgnored[dir] = true
		}
	}
	var kept []string
	for _, rel := range ignored {
		if !notIgnored[rel] {
			kept = append(kept, rel)
		}
	}
	return p.join(kept), nil
}

// others runs `git ls-files --others --directory` and returns slash-separated
// relative names without the trailing slash Git puts on directories.
func (p Project) others(flags ...string) ([]string, error) {
	out, err := p.git(append([]string{"ls-files", "-z", "--others", "--directory"}, flags...)...)
	if err != nil {
		return nil, err
	}
	var rels []string
	for _, rel := range bytes.Split(out, []byte{0}) {
		if len(rel) > 0 {
			rels = append(rels, strings.TrimSuffix(string(rel), "/"))
		}
	}
	return rels, nil
}

func (p Project) join(rels []string) []string {
	var paths []string
	for _, rel := range rels {
		paths = append(paths, filepath.Join(p.Root, filepath.FromSlash(rel)))
	}
	return paths
}

// WorkDir is the project's private .obit directory.
func (p Project) WorkDir() string { return filepath.Join(p.Root, ".obit") }

// Ensure creates .obit and lists it in the repository's info/exclude, so it
// never shows up in `git status` and is never committed.
func (p Project) Ensure() error {
	if err := os.MkdirAll(p.WorkDir(), 0o755); err != nil {
		return err
	}
	out, err := p.git("rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return err
	}
	exclude := strings.TrimSpace(string(out))
	if !filepath.IsAbs(exclude) {
		exclude = filepath.Join(p.Root, exclude)
	}
	data, err := os.ReadFile(exclude)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if t := strings.TrimSpace(line); t == "/.obit/" || t == ".obit/" || t == "/.obit" || t == ".obit" {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(exclude), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(exclude, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	prefix := ""
	if len(data) > 0 && !bytes.HasSuffix(data, []byte("\n")) {
		prefix = "\n"
	}
	_, err = f.WriteString(prefix + "# Obituary's private working directory\n/.obit/\n")
	return err
}
