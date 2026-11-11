package project

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// ErrIndexSnapshotUnavailable selects the existing uncached Git query. It
// never authorizes reuse of an answer whose dependencies could not be bounded.
var ErrIndexSnapshotUnavailable = errors.New("Git metadata requires a fresh query") //nolint:staticcheck // ST1005: Git is a proper noun.

// Configuration can change cwd-to-index path mapping without rewriting index
// (core.worktree). Git var supplies even absent default config paths; never
// guess the user's config directories. Includes/unknown origins use uncached
// queries instead of introducing another Git configuration resolver here.
func (p Project) indexConfiguration(ctx context.Context, gitdir, common string) ([]indexFile, error) {
	paths := []string{filepath.Join(common, "config"), filepath.Join(gitdir, "config.worktree"), filepath.Join(gitdir, "HEAD")}
	for _, name := range []string{"GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM"} {
		out, err := p.gitContext(ctx, "var", name)
		if err != nil {
			var exited *exec.ExitError
			// A disabled Git path variable has exit 1 and no diagnostic.
			if errors.As(err, &exited) && exited.ExitCode() == 1 && len(exited.Stderr) == 0 {
				continue
			}
			return nil, err
		}
		for _, path := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
			path = strings.TrimSuffix(path, "\r")
			if path != "" && path != "/dev/null" {
				paths = append(paths, path)
			}
		}
	}
	known := make(map[string]bool)
	var files []indexFile
	for _, path := range paths {
		path, err := pathutil.Abs(path)
		if err != nil {
			return nil, err
		}
		if known[path] {
			continue
		}
		file, err := indexMetadata(path, true)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
		known[path] = true
	}
	out, err := p.gitContext(ctx, "config", "--list", "--name-only", "--show-origin", "--null", "--includes")
	if err != nil {
		return nil, err
	}
	if err := checkIndexConfigOrigins(out, p.Root, known); err != nil {
		return nil, err
	}
	if same, err := sameIndexFiles(ctx, files); err != nil {
		return nil, err
	} else if !same {
		return nil, errIndexChanged
	}
	return files, nil
}

func checkIndexConfigOrigins(out []byte, root string, known map[string]bool) error {
	parts := bytes.Split(out, []byte{0})
	if len(parts)%2 != 1 || len(parts[len(parts)-1]) != 0 {
		return errors.New("unexpected Git configuration origin reply")
	}
	for i := 0; i+1 < len(parts); i += 2 {
		origin, key := string(parts[i]), string(parts[i+1])
		if strings.HasPrefix(key, "include.") || strings.HasPrefix(key, "includeif.") || !strings.HasPrefix(origin, "file:") {
			return ErrIndexSnapshotUnavailable
		}
		path := strings.TrimPrefix(origin, "file:")
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		path, err := pathutil.Abs(path)
		if err != nil {
			return err
		}
		if !known[path] {
			return ErrIndexSnapshotUnavailable
		}
	}
	return nil
}
