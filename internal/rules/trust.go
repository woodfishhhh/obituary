package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

// Trust requires review before loading project-local rules: a repository
// can ship an .obitrules that targets the user's own untracked
// files, so a project's rules are skipped (not loaded with a warning) until
// the user reviews them with `obit trust`. Trust pins the file's SHA-256; any
// change needs a new review. OBIT_TRUST_PROJECT_RULES=1 overrides for CI.

// Trust states of a project rules file.
const (
	Absent      = "absent"
	Untrusted   = "untrusted"
	Trusted     = "trusted"
	Changed     = "changed"         // trusted once, edited since
	Missing     = "missing-trusted" // trusted once, removed since
	EnvOverride = "env-override"
)

// FileState is the trust state of one rules file.
type FileState struct {
	Path    string `json:"path"`
	Status  string `json:"status"`
	SHA256  string `json:"sha256,omitempty"`
	Warning string `json:"warning,omitempty"`
	content string
}

// TrustStore is trusted_rules.json in the data directory.
type TrustStore struct {
	Path string
}

// Inspect checks readability and decoding without changing the trust store.
func (t TrustStore) Inspect() error {
	_, err := t.read()
	return err
}

type trustEntry struct {
	SHA256    string    `json:"sha256"`
	TrustedAt time.Time `json:"trustedAt"`
}

type trustFile struct {
	Version int                   `json:"version"`
	Trusted map[string]trustEntry `json:"trusted"`
}

// Reuse the handle-verified long-name resolver. Unicode folding is not the
// Windows name equivalence relation and must never grant another file trust.
func rulePath(path string) (string, error) {
	abs, err := pathutil.Abs(path)
	if err != nil {
		return "", err
	}
	return (winfs.FS{}).RealPath(abs)
}

func key(path string) string { return path }

func digest(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func (t TrustStore) read() (trustFile, error) {
	f := trustFile{Version: 2, Trusted: map[string]trustEntry{}}
	data, err := os.ReadFile(t.Path)
	if errors.Is(err, os.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	f.Version = 1 // an existing unversioned file predates canonical keys
	if err := json.Unmarshal(data, &f); err != nil {
		return f, fmt.Errorf("%s is corrupt: %w", t.Path, err)
	}
	if f.Trusted == nil {
		f.Trusted = map[string]trustEntry{}
	}
	if f.Version != 1 && f.Version != 2 {
		return f, fmt.Errorf("unsupported trust store version %d; review rules with obit trust", f.Version)
	}
	if f.Version == 1 {
		// v1 lost Unicode spelling. Even a unique present-day match cannot
		// prove it is the repository the user reviewed before a rename/delete.
		f.Trusted = map[string]trustEntry{}
	}
	return f, nil
}

func (t TrustStore) write(f trustFile) error {
	f.Version = 2
	if err := os.MkdirAll(filepath.Dir(t.Path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(t.Path), "trusted-*.tmp")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), t.Path)
}

// Check reports whether the rules file at path may be loaded.
func (t TrustStore) Check(path string) (FileState, error) {
	state := FileState{Path: path}
	real, err := rulePath(path)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, core.ErrNotFound) {
		return t.missingState(path)
	}
	if err != nil {
		return state, err
	}
	state.Path = real
	content, ok, err := readFile(real)
	if err != nil {
		return state, err
	}
	if !ok {
		return t.missingState(real)
	}
	state.content, state.SHA256 = content, digest(content)
	if v := os.Getenv("OBIT_TRUST_PROJECT_RULES"); v == "1" || strings.EqualFold(v, "true") {
		state.Status = EnvOverride
		return state, nil
	}
	f, err := t.read()
	if err != nil {
		return state, err
	}
	if f.Version == 1 {
		state.Warning = "trust store format was upgraded; review the rules and run obit trust again"
	}
	entry, ok := f.Trusted[key(real)]
	switch {
	case !ok:
		state.Status = Untrusted
	case entry.SHA256 == state.SHA256:
		state.Status = Trusted
	default:
		state.Status = Changed
	}
	return state, nil
}

// Trust pins the content of the rules file at path. reviewed is the SHA-256
// of the content the user actually looked at: if the file changed since,
// nothing is trusted. Pass "" only where no review step exists (tests).
func (t TrustStore) Trust(path, reviewed string) (FileState, error) {
	var state FileState
	err := t.locked(func() error {
		var err error
		if state, err = t.Check(path); err != nil {
			return err
		}
		if state.Status == Absent || state.Status == Missing {
			return fmt.Errorf("%s does not exist", path)
		}
		if reviewed != "" && state.SHA256 != reviewed {
			return fmt.Errorf("%s changed after it was shown for review; nothing was trusted, run obit trust again", path)
		}
		if _, err := Parse(state.content, path); err != nil {
			return err
		}
		f, err := t.read()
		if err != nil {
			return err
		}
		f.Trusted[key(state.Path)] = trustEntry{SHA256: state.SHA256, TrustedAt: time.Now()}
		if err := t.write(f); err != nil {
			return err
		}
		state.Status = Trusted
		state.Warning = ""
		return nil
	})
	return state, err
}

// The existing trust entry is also the deletion tombstone. Only explicit
// Untrust removes it, so a checkout cannot silently restore builtin rules.
func (t TrustStore) missingState(path string) (FileState, error) {
	state := FileState{Path: path, Status: Absent}
	f, err := t.read()
	if err != nil {
		return state, err
	}
	real, err := revocationPath(path)
	if err != nil {
		return state, err
	}
	state.Path = real
	for saved := range f.Trusted {
		same := saved == real
		if !same {
			same, err = winfs.SameDirectoryEntry(real, saved)
			if err != nil {
				return state, err
			}
		}
		if same {
			state.Status = Missing
			break
		}
	}
	return state, nil
}

// locked serialises read-modify-write cycles on the trust store across
// processes, so a concurrent untrust can never be undone by a stale write.
func (t TrustStore) locked(fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(t.Path), 0o700); err != nil {
		return err
	}
	release, err := core.LockFile(t.Path+".lock", true)
	if err != nil {
		return err
	}
	defer release()
	return fn()
}

// Untrust forgets the rules file at path; it reports whether it was trusted.
func (t TrustStore) Untrust(path string) (bool, error) {
	real, err := revocationPath(path)
	if err != nil {
		return false, err
	}
	id, idErr := winfs.IdentifyFollowing(real)
	if idErr != nil && !errors.Is(idErr, os.ErrNotExist) && !errors.Is(idErr, core.ErrNotFound) {
		return false, idErr
	}
	was := false
	err = t.locked(func() error {
		f, err := t.read()
		if err != nil {
			return err
		}
		for saved := range f.Trusted {
			resolved, err := revocationPath(saved)
			samePath := err == nil && resolved == real
			other, otherErr := winfs.IdentifyFollowing(saved)
			sameID := idErr == nil && otherErr == nil && id.Serial == other.Serial && id.FileID == other.FileID
			sameMissing := false
			if idErr != nil && (errors.Is(otherErr, os.ErrNotExist) || errors.Is(otherErr, core.ErrNotFound)) {
				sameMissing, err = winfs.SameDirectoryEntry(real, saved)
				if err != nil {
					return err
				}
			}
			if saved == real || samePath || sameID || sameMissing {
				delete(f.Trusted, saved)
				was = true
			}
		}
		return t.write(f)
	})
	return was, err
}

// Revocation must also work after a checkout removes the rules. Resolve the
// surviving parent for aliases; never use Unicode folding to match a name.
func revocationPath(path string) (string, error) {
	real, err := rulePath(path)
	if err == nil || !errors.Is(err, os.ErrNotExist) && !errors.Is(err, core.ErrNotFound) {
		return real, err
	}
	abs, err := pathutil.Abs(path)
	if err != nil {
		return "", err
	}
	if parent, err := winfs.Resolve(filepath.Dir(abs)); err == nil {
		return filepath.Join(parent, filepath.Base(abs)), nil
	}
	return abs, nil
}

// Content returns the rules text read by Check (empty when absent).
func (s FileState) Content() string { return s.content }
