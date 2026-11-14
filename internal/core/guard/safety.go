package guard

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/core/internal/pathcheck"
	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// Guard decides which paths may be given a lifespan. It is checked both when
// an item is added and again right before it is recycled.
type Guard struct {
	// Keep holds locations that must survive: they and every ancestor are refused.
	Keep []string
	// WatchAllowed permits a folder policy on a protected container (Downloads).
	// It never exempts the container itself from Check or any system exclusion.
	WatchAllowed []string
	// System holds trees that are refused together with everything inside them.
	System []string
	// Canonical resolves a path to its real location (short names expanded,
	// links in parent folders resolved). Nil means paths are compared as given.
	Canonical func(string) (string, error)
	// WalkLimit bounds how many entries are inspected when looking for
	// repositories inside a folder; 0 means the default.
	WalkLimit int
}

// RefusedError reports a path the guard will not touch.
type RefusedError struct {
	Path   string
	Reason string
	// Forceable is true when --force overrides the refusal.
	Forceable bool
}

func (e *RefusedError) Error() string {
	if e.Forceable {
		return fmt.Sprintf("refusing %s: %s (use --force to override)", e.Path, e.Reason)
	}
	return fmt.Sprintf("refusing %s: %s", e.Path, e.Reason)
}

const defaultWalkLimit = 200_000

var errWalkLimit = errors.New("walk limit reached")

// DefaultGuard builds the guard from the current user's environment. keep adds
// further locations, such as known folders that were redirected elsewhere.
func DefaultGuard(home string, keep ...string) Guard {
	env := os.Getenv
	profile := env("USERPROFILE")
	keep = append(keep, profile, env("APPDATA"), env("LOCALAPPDATA"), env("PUBLIC"), env("OneDrive"), home)
	if profile != "" {
		for _, name := range []string{"Desktop", "Documents", "Downloads", "Pictures", "Videos", "Music", "AppData", "OneDrive"} {
			keep = append(keep, filepath.Join(profile, name))
		}
	}
	system := []string{
		env("SystemRoot"), env("ProgramFiles"), env("ProgramFiles(x86)"), env("ProgramW6432"), env("ProgramData"), home,
	}
	return Guard{Keep: cleanAll(keep), System: cleanAll(system)}
}

// WithCanonical returns a copy of g that also compares real locations, adding
// the canonical form of every protected root.
func (g Guard) WithCanonical(canonical func(string) (string, error)) Guard {
	expand := func(roots []string) []string {
		out := append([]string(nil), roots...)
		for _, root := range roots {
			if real, err := canonical(root); err == nil && !pathcheck.SamePath(real, root) {
				out = append(out, filepath.Clean(real))
			}
		}
		return out
	}
	g.Keep, g.System, g.Canonical = expand(g.Keep), expand(g.System), canonical
	return g
}

func cleanAll(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if p != "" {
			out = append(out, filepath.Clean(p))
		}
	}
	return out
}

// Check validates an absolute, cleaned path. force only overrides forceable refusals.
func (g Guard) Check(path string, force bool) error {
	return g.CheckContext(context.Background(), path, force)
}

// CheckContext keeps a recursive check inside its caller's work budget.
// An interrupted walk is never evidence that the directory is safe.
func (g Guard) CheckContext(ctx context.Context, path string, force bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	refuse := func(reason string, forceable bool) error {
		if forceable && force {
			return nil
		}
		return &RefusedError{Path: path, Reason: reason, Forceable: forceable}
	}
	if !filepath.IsAbs(path) {
		return refuse("path is not absolute", false)
	}
	candidates := []string{path}
	if g.Canonical != nil {
		real, err := g.Canonical(path)
		if err != nil {
			return fmt.Errorf("resolve real location of %s: %w", path, err)
		}
		candidates = append(candidates, filepath.Clean(real))
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, p := range candidates {
		if reason := g.ProtectedReason(p); reason != "" {
			return refuse(reason, false)
		}
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if pathcheck.IsLink(info) {
		return refuse("it is a link or junction; give the real location instead", false)
	}
	// Force passes a contained repository and an unfinished walk alike, so a
	// forced check skips the walk (Explorer labels force every tracked folder).
	if info.IsDir() && !force {
		switch found, err := g.findRepository(ctx, path); {
		case err != nil:
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("check %s for .git repositories: %w", path, err)
		case found != "":
			return refuse("it contains a .git repository ("+found+")", true)
		}
	}
	return ctx.Err()
}

// ProtectedReason reports protected locations without walking the target.
func (g Guard) ProtectedReason(path string) string {
	if isVolumeRoot(path) {
		return "it is a drive or share root"
	}
	if graves := filepath.Join(pathutil.VolumeRoot(path), ".obituary"); pathcheck.SamePath(path, graves) || pathcheck.IsInside(path, graves) {
		return "it is inside Obituary's graveyard (" + graves + ")"
	}
	for _, root := range g.System {
		if pathcheck.SamePath(path, root) || pathcheck.IsInside(path, root) {
			return "it is inside a system or Obituary location (" + root + ")"
		}
	}
	for _, kept := range g.Keep {
		if pathcheck.SamePath(path, kept) || pathcheck.IsInside(kept, path) {
			return "it is or contains a protected folder (" + kept + ")"
		}
	}
	return ""
}

// findRepository walks dir (without following links) and returns the first
// .git entry found. It fails if the walk errors or exceeds the limit, because
// an incomplete check must not count as a pass.
func (g Guard) findRepository(ctx context.Context, dir string) (string, error) {
	limit := g.WalkLimit
	if limit <= 0 {
		limit = defaultWalkLimit
	}
	var found string
	seen := 0
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			return err
		}
		if seen++; seen > limit {
			return errWalkLimit
		}
		// WalkDir does not descend into symlinks or junctions, so linked trees are not searched.
		if strings.EqualFold(d.Name(), ".git") && p != dir {
			found = p
			return fs.SkipAll
		}
		return nil
	})
	if errors.Is(err, errWalkLimit) {
		return "", fmt.Errorf("more than %d entries", limit)
	}
	return found, err
}

func isVolumeRoot(path string) bool {
	return strings.TrimRight(path, `\/`) == strings.TrimRight(pathutil.VolumeRoot(path), `\/`)
}
