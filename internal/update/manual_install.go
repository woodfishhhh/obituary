package update

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
)

// rename is os.Rename; tests replace it to fail a step part-way.
var rename = os.Rename

// swap is the transaction for replacing a manual install's programs. It lives
// in a folder of its own beside them (a rename never leaves the volume):
//
//	.obituary-update-XXXX/new  the verified programs, staged before anything moves
//	.obituary-update-XXXX/old  the previous programs, kept until their lifespan ends
//
// Nothing is deleted. Rolling back moves every file back by name.
type swap struct {
	dir, root, staged, kept string
	names                   []string
	moved, placed           []string
	stranded                bool // a rollback step failed; the folder holds the only copy
}

func newSwap(dir string, names []string) (*swap, error) {
	root, err := os.MkdirTemp(dir, ".obituary-update-")
	if err != nil {
		return nil, err
	}
	s := &swap{dir: dir, root: root, staged: filepath.Join(root, "new"), kept: filepath.Join(root, "old"), names: names}
	for _, sub := range []string{s.staged, s.kept} {
		if err := os.Mkdir(sub, 0o700); err != nil {
			s.release()
			return nil, err
		}
	}
	return s, nil
}

// release removes the empty folders newSwap made when it could not finish.
func (s *swap) release() {
	for _, path := range []string{s.staged, s.kept, s.root} {
		_ = os.Remove(path)
	}
}

// commit saves each current program under old/, then moves the new ones in.
func (s *swap) commit() error {
	for _, name := range s.names {
		err := rename(filepath.Join(s.dir, name), filepath.Join(s.kept, name))
		if errors.Is(err, fs.ErrNotExist) {
			continue // a program the old install did not have
		}
		if err != nil {
			return fmt.Errorf("set aside %s: %w", name, err)
		}
		s.moved = append(s.moved, name)
	}
	for _, name := range s.names {
		if err := rename(filepath.Join(s.staged, name), filepath.Join(s.dir, name)); err != nil {
			return fmt.Errorf("install %s: %w", name, err)
		}
		s.placed = append(s.placed, name)
	}
	return nil
}

// abort undoes commit and returns cause with what happened to the files.
func (s *swap) abort(cause error) error {
	var failed error
	for _, name := range slices.Backward(s.placed) {
		failed = errors.Join(failed, rename(filepath.Join(s.dir, name), filepath.Join(s.staged, name)))
	}
	for _, name := range slices.Backward(s.moved) {
		failed = errors.Join(failed, rename(filepath.Join(s.kept, name), filepath.Join(s.dir, name)))
	}
	if failed != nil {
		s.stranded = true
		return fmt.Errorf("%w; the previous files could not all be put back and are in %s: %w", cause, s.kept, failed)
	}
	return fmt.Errorf("%w; the previous version was put back", cause)
}

// installManual replaces the programs of a manual install. The download and
// every check happen before anything moves; the window may close only after.
func (p plan) installManual(ctx context.Context, s session) (installed string, err error) {
	ctx, cancel := context.WithTimeout(ctx, installTimeout)
	defer cancel()
	names := programs(runtime.GOOS, runtime.GOARCH)
	name, err := p.archive()
	if err != nil {
		return "", err
	}
	data, err := p.fetchArchive(ctx, s, name)
	if err != nil {
		return "", err
	}
	tx, err := newSwap(filepath.Dir(s.exe), names)
	if err != nil {
		return "", fmt.Errorf("prepare update folder: %w", err)
	}
	defer func() { s.retire(tx) }()
	if err := extractPrograms(data, name, names, tx.staged); err != nil {
		return "", err
	}
	s.progress.phase(PhaseReady)
	if err := s.wait(); err != nil {
		return "", err
	}
	if err := p.verifyBeforeInstall(ctx, s.exe, s.current, s.log, s.run); err != nil {
		return "", err
	}
	if err := s.stop(); err != nil {
		return "", err
	}
	fmt.Fprintf(s.log, "Updating the programs in %s from %s to %s: %v\n", tx.dir, s.current, p.version, names)
	s.progress.phase(PhaseInstalling)
	if err := tx.commit(); err != nil {
		return "", tx.abort(err)
	}
	if installed, err = p.verify(ctx, s.exe, s.current, s.log, s.run); err != nil {
		return "", tx.abort(err)
	}
	return installed, nil
}

// fetchArchive downloads the release archive and checks it against the
// checksums.txt of the same release before anything is unpacked.
func (p plan) fetchArchive(ctx context.Context, s session, name string) ([]byte, error) {
	s.progress.phase(PhaseDownloading)
	sums, dir, err := s.src.checksums(ctx, s.log)
	if err != nil {
		return nil, err
	}
	want, err := checksumFor(sums, name)
	if err != nil {
		return nil, err
	}
	data, err := s.src.get(ctx, dir, name, archiveLimit, s.progress.bytes)
	if err != nil {
		return nil, err
	}
	s.progress.phase(PhaseVerifying)
	if err := verifySum(data, want, name); err != nil {
		return nil, err
	}
	fmt.Fprintf(s.log, "SHA-256 verified: %s\n", name)
	return data, nil
}
