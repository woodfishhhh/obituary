package gitclean

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	removecmd "github.com/woodfishhhh/obituary/internal/cli/remove"
	"github.com/woodfishhhh/obituary/internal/guardrm"
)

func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use: "git-clean [options] [--] [pathspec]...", Short: "Bury the paths selected by git clean's dry run",
		DisableFlagParsing: true,
		RunE:               run,
	}
}

func run(_ *cobra.Command, args []string) error {
	opts, err := guardrm.ParseClean(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "git-clean:", err)
		return cmdutil.ErrReported
	}
	for _, ref := range opts.Paths {
		if err := removecmd.CheckParentTraversal(ref); err != nil {
			fmt.Fprintf(os.Stderr, "git-clean: cannot remove '%s': %v\n", ref, err)
			return cmdutil.ErrReported
		}
	}
	root, prefix, err := cleanWorktree()
	if err != nil {
		fmt.Fprintln(os.Stderr, "git-clean:", err)
		return cmdutil.ErrReported
	}
	if opts.DryRun {
		// Always force dry run too, even when -n occurs after a pathspec.
		return cleanPreview(append([]string{"-n"}, args...), false, nil)
	}
	if opts.Force == 0 {
		required, err := cleanRequiresForce()
		if err != nil || required {
			if err == nil {
				err = errors.New("clean.requireForce is true (default); an explicit -f is required")
			}
			fmt.Fprintln(os.Stderr, "git-clean:", err)
			return cmdutil.ErrReported
		}
	}
	var preview bytes.Buffer
	if err := cleanPreview(append([]string{"-n"}, opts.Preview...), true, &preview); err != nil {
		return err
	}
	paths, err := cleanPaths(preview.String(), root, prefix)
	if err != nil {
		fmt.Fprintln(os.Stderr, "git-clean:", err)
		return cmdutil.ErrReported
	}
	if len(paths) == 0 {
		return nil
	}
	svc, err := cmdutil.Open()
	if err != nil {
		return err
	}
	// Git's second -f explicitly permits nested repositories. Its first
	// -f only enables cleaning, so it must not bypass Obituary's Guard.
	return removecmd.RemovePaths(svc, guardrm.RemoveOptions{
		Paths: paths, Recursive: true, Force: opts.Force > 1, Verbose: !opts.Quiet,
	}, "git-clean", false)
}

// Resolve Git's worktree in exactly the environment used by the preview.
// GIT_DIR/GIT_WORK_TREE and core.worktree may select a different checkout.
func cleanWorktree() (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel", "--show-prefix")
	hideWindow(cmd)
	cmd.Env = cleanEnvironment()
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("resolve git worktree: %w", err)
	}
	lines := strings.Split(strings.TrimSuffix(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n"), "\n")
	if len(lines) != 2 || !filepath.IsAbs(lines[0]) {
		return "", "", errors.New("invalid git worktree/prefix")
	}
	root, err := filepath.EvalSymlinks(lines[0])
	if err != nil {
		return "", "", err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	cwd, err = filepath.EvalSymlinks(cwd)
	if err != nil {
		return "", "", err
	}
	if !withinCleanWorktree(root, cwd) {
		return "", "", errors.New("current directory is outside the git worktree")
	}
	base := filepath.Join(root, filepath.FromSlash(lines[1]))
	if !withinCleanWorktree(root, base) || !strings.EqualFold(filepath.Clean(base), filepath.Clean(cwd)) {
		return "", "", errors.New("git worktree prefix does not match the current directory")
	}
	return root, lines[1], nil
}

func withinCleanWorktree(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func cleanEnvironment() []string { return append(os.Environ(), "LC_ALL=C", "LANG=C") }

func cleanRequiresForce() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "config", "--type=bool", "--get", "clean.requireForce")
	hideWindow(cmd)
	cmd.Env = cleanEnvironment()
	out, err := cmd.Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return true, nil
	} // unset: Git's default
	if err != nil {
		return true, fmt.Errorf("read clean.requireForce: %w", err)
	}
	return strings.TrimSpace(string(out)) != "false", nil
}

func cleanPreview(args []string, parse bool, out *bytes.Buffer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	gitArgs := []string{"clean"}
	if parse {
		gitArgs = []string{"-c", "core.quotePath=true", "-c", "color.ui=false", "clean"}
	}
	cmd := exec.CommandContext(ctx, "git", append(gitArgs, args...)...)
	hideWindow(cmd)
	cmd.Env = cleanEnvironment()
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if out != nil {
		cmd.Stdout = out
	}
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "git-clean: preview failed:", err)
		return cmdutil.ErrReported
	}
	return nil
}

// Git has no NUL-delimited clean preview. Its C-style quoting is mandatory
// above; malformed/unrecognized output invalidates the whole list before Bury.
func cleanPaths(preview, root, prefix string) ([]string, error) {
	var paths []string
	for _, line := range strings.Split(strings.TrimSuffix(preview, "\n"), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" || strings.HasPrefix(line, "Would skip repository ") {
			continue
		}
		path, ok := strings.CutPrefix(line, "Would remove ")
		if !ok {
			return nil, fmt.Errorf("unrecognized git clean preview: %q", line)
		}
		if strings.HasPrefix(path, `"`) {
			var err error
			path, err = strconv.Unquote(path)
			if err != nil {
				return nil, fmt.Errorf("unrecognized quoted path: %w", err)
			}
		}
		path = filepath.Clean(filepath.FromSlash(path))
		if filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
			return nil, fmt.Errorf("preview path is not relative: %q", path)
		}
		path = filepath.Join(root, prefix, path)
		if !withinCleanWorktree(root, path) || strings.EqualFold(path, root) {
			return nil, fmt.Errorf("preview path is outside the git worktree: %q", path)
		}
		paths = append(paths, path)
	}
	return paths, nil
}
