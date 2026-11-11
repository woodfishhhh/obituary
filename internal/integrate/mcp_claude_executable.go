package integrate

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"github.com/woodfishhhh/obituary/internal/project"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

// resolveClaudeExecutable never passes a script to CreateProcess or a shell.
// npm's wrappers have a fixed native sibling layout; their contents are not run.
func resolveClaudeExecutable() (string, error) {
	regular := func(path string) bool {
		info, err := os.Stat(path)
		return err == nil && info.Mode().IsRegular()
	}
	if path, err := claudeLookPath("claude.exe"); errors.Is(err, exec.ErrDot) {
		return "", fmt.Errorf("refusing Claude executable from the current directory: %w", err)
	} else if err == nil && regular(path) {
		return checkedClaudeExecutable(path)
	}
	// LookPath on Windows may not recognize extensionless npm shell scripts
	// or .ps1 shims unless PATHEXT includes them. Inspect only these fixed names.
	var refused []string
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		// Match Go's ErrDot policy, including relative directories other than dot.
		// https://go.dev/blog/path-security
		if !filepath.IsAbs(dir) {
			refused = append(refused, fmt.Sprintf("refusing non-absolute PATH entry %q", dir))
			continue
		}
		for _, name := range []string{"claude.cmd", "claude", "claude.ps1"} {
			if !regular(filepath.Join(dir, name)) {
				continue
			}
			native := filepath.Join(dir, "node_modules", "@anthropic-ai", "claude-code", "bin", "claude.exe")
			if regular(native) {
				return checkedClaudeExecutable(native)
			}
		}
	}
	if len(refused) != 0 {
		return "", errors.New(strings.Join(refused, "; "))
	}
	return "", errors.New("could not find a native claude.exe (including npm layout)")
}

func checkedClaudeExecutable(path string) (checked string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("refusing Claude executable %q: %w", path, err)
		}
	}()
	if !filepath.IsAbs(path) {
		return "", errors.New("path is not absolute")
	}
	real, err := winfs.Resolve(path)
	if err != nil {
		return "", err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	realCWD, err := winfs.Resolve(cwd)
	if err != nil {
		return "", err
	}
	rootPaths := []string{cwd, realCWD}
	for _, dir := range []string{cwd, realCWD} {
		proj, found, err := project.Find(dir)
		if err != nil {
			return "", err
		}
		if found {
			rootPaths = append(rootPaths, proj.Root)
		}
	}
	var roots []claudeExecutableRoot
	for _, dir := range rootPaths {
		id, err := winfs.IdentifyFollowing(dir)
		if err != nil {
			return "", fmt.Errorf("identify protected directory %q: %w", dir, err)
		}
		roots = append(roots, claudeExecutableRoot{path: dir, id: id, cwd: dir == cwd || dir == realCWD})
	}
	// Check both spellings: a project junction pointing out still belongs to
	// the project, and an external junction pointing in is equally untrusted.
	for _, candidate := range []string{path, real} {
		if err := checkClaudeAncestors(candidate, roots); err != nil {
			return "", err
		}
	}
	names, err := winfs.HardlinkNames(real)
	if err != nil {
		return "", err
	}
	for _, name := range names {
		if err := checkClaudeAncestors(name, roots); err != nil {
			return "", err
		}
	}
	return real, nil
}

type claudeExecutableRoot struct {
	path string
	id   core.Identity
	cwd  bool
}

// Compare handle identities so path aliases cannot bypass executable checks.
// The winfs identity uses FILE_ID_INFO (128 bits, including ReFS).
func checkClaudeAncestors(path string, roots []claudeExecutableRoot) error {
	for dir := pathutil.Parent(path); ; dir = pathutil.Parent(dir) {
		id, err := winfs.IdentifyFollowing(dir)
		if err != nil {
			return fmt.Errorf("identify executable ancestor %q: %w", dir, err)
		}
		for _, root := range roots {
			if id.Serial == root.id.Serial && id.FileID == root.id.FileID {
				hint := ""
				if root.cwd {
					hint = "; run obit init from a directory that does not contain your Claude installation"
				}
				return fmt.Errorf("inside the current directory or Git project %q via %q%s", root.path, dir, hint)
			}
		}
		if pathutil.Parent(dir) == dir {
			return nil
		}
	}
}

var claudeVersionPattern = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?: \(Claude Code\))?$`)

func claudeHasConfigLock(output []byte) bool {
	parts := claudeVersionPattern.FindStringSubmatch(strings.TrimSpace(string(output)))
	if parts == nil {
		return false
	}
	var version [3]int
	for i := range version {
		n, err := strconv.Atoi(parts[i+1])
		if err != nil {
			return false
		}
		version[i] = n
	}
	return version[0] > 2 || version[0] == 2 && (version[1] > 1 || version[1] == 1 && version[2] >= 318)
}
