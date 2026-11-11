package project

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/pathutil"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

var errIndexChanged = errors.New("Git index changed during verification; retry") //nolint:staticcheck // ST1005: Git is a proper noun.

type indexFile struct {
	path   string
	stamp  winfs.MetadataStamp
	absent bool
}

func indexMetadata(path string, optional bool) (indexFile, error) {
	f := indexFile{path: path}
	stamp, err := winfs.ReadMetadata(path)
	if optional && errors.Is(err, os.ErrNotExist) {
		f.absent = true
		return f, nil
	}
	if err != nil {
		return f, err
	}
	if stamp.Directory {
		// Directory contents change when Git publishes an index. Only its
		// identity controls routing; file versions retain size and time.
		stamp.Size, stamp.Modified = 0, 0
	}
	f.stamp = stamp
	return f, nil
}

func sameIndexFiles(ctx context.Context, files []indexFile) (bool, error) {
	for _, old := range files {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		current, err := indexMetadata(old.path, true)
		if err != nil || current != old {
			return false, err
		}
	}
	return true, nil
}

// IndexObserver is scoped to one sweep. Git, rather than a private index
// parser, resolves worktree/submodule paths and interprets split/sparse indexes.
// Git v2.54.0 read-cache.c commits index.lock by rename; split shared files
// are immutable content, but reads refresh their mtime (freshen_shared_index).
type IndexObserver struct {
	// Tests publish real Git metadata before this observer accepts its snapshot.
	afterIndexQuery func()
	project         Project
	routes          []indexFile
	index           string
	gitdir          string
	config          []indexFile
	format          indexFile
	uncached        bool
}

// ObserveIndex resolves metadata outside the ledger lock. TrackedSnapshot can
// also run there; Current must revalidate its result inside the moving transaction.
func (p Project) ObserveIndex(ctx context.Context) (*IndexObserver, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entry, err := indexMetadata(filepath.Join(p.Root, ".git"), false)
	if err != nil {
		return nil, err
	}
	out, err := p.gitContext(ctx, "rev-parse", "--path-format=absolute", "--absolute-git-dir", "--git-path", "index", "--git-common-dir")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(lines) != 3 {
		return nil, errors.New("unexpected Git index path reply")
	}
	for i := range lines {
		lines[i], err = pathutil.Abs(strings.TrimSuffix(lines[i], "\r"))
		if err != nil {
			return nil, err
		}
	}
	dir, err := indexMetadata(lines[0], false)
	if err != nil {
		return nil, err
	}
	common, err := indexMetadata(filepath.Join(lines[0], "commondir"), true)
	if err != nil {
		return nil, err
	}
	o := &IndexObserver{project: p, routes: []indexFile{entry}, index: lines[1], gitdir: lines[0]}
	if dir.path != entry.path {
		o.routes = append(o.routes, dir)
	}
	o.routes = append(o.routes, common)
	o.config, err = p.indexConfiguration(ctx, lines[0], lines[2])
	if err != nil {
		return nil, err
	}
	if same, err := sameIndexFiles(ctx, o.routes); err != nil {
		return nil, err
	} else if !same {
		return nil, errIndexChanged
	}
	return o, nil
}

// IndexSnapshot is authority only while every routing and index file still
// matches. Current is called after all other checks, immediately before move.
type IndexSnapshot struct{ files []indexFile }

func (s *IndexSnapshot) Current(ctx context.Context) (bool, error) {
	return sameIndexFiles(ctx, s.files)
}

func (o *IndexObserver) TrackedSnapshot(ctx context.Context, paths []string) ([]bool, *IndexSnapshot, error) {
	return o.trackedSnapshot(ctx, paths, false)
}

// TrackedRealSnapshot accepts names already returned by FS.RealPath and Find.
// Reuse those checked names inside the final Git budget; expanding up to
// 128 filesystem names again would consume it. Index and routing checks stay live.
func (o *IndexObserver) TrackedRealSnapshot(ctx context.Context, paths []string) ([]bool, *IndexSnapshot, error) {
	return o.trackedSnapshot(ctx, paths, true)
}

func (o *IndexObserver) trackedSnapshot(ctx context.Context, paths []string, realNames bool) ([]bool, *IndexSnapshot, error) {
	if err := o.refreshRoutes(ctx); err != nil {
		return nil, nil, err
	}
	before, err := indexMetadata(o.index, true)
	if err != nil {
		return nil, nil, err
	}
	if err := o.checkIndexFormat(ctx, before); err != nil {
		return nil, nil, err
	}
	files := append([]indexFile(nil), o.routes...)
	files = append(files, o.config...)
	shared, err := o.sharedFiles(ctx)
	if err != nil {
		return nil, nil, err
	}
	// More than eight shared versions would make every final metadata check
	// too expensive. Keep the existing fresh-per-move Git behavior there.
	if len(shared) > 8 {
		o.uncached = true
		return nil, nil, ErrIndexSnapshotUnavailable
	}
	digests := make([][32]byte, len(shared))
	for i, file := range shared {
		digests[i], err = indexDigest(ctx, file.path)
		if err != nil {
			return nil, nil, err
		}
	}
	tracked, err := o.project.trackedBatchContext(ctx, paths, realNames)
	if err != nil {
		return nil, nil, err
	}
	if o.afterIndexQuery != nil {
		o.afterIndexQuery()
	}
	after, err := o.checkSharedFiles(ctx, shared, digests)
	if err != nil {
		return nil, nil, err
	}
	files = append(files, after...)
	// Check the main index last: a split-index relink invalidates the entire
	// answer even when the previously resolved shared file still exists.
	files = append(files, before)
	if same, err := sameIndexFiles(ctx, files); err != nil {
		return nil, nil, err
	} else if !same {
		return nil, nil, errIndexChanged
	}
	o.format = before
	return tracked, &IndexSnapshot{files: files}, nil
}

func (o *IndexObserver) checkIndexFormat(ctx context.Context, before indexFile) error {
	if before.absent || before == o.format {
		return nil
	}
	_, err := indexDigest(ctx, o.index)
	if errors.Is(err, ErrIndexSnapshotUnavailable) {
		o.uncached = true
	}
	return err
}

func (o *IndexObserver) refreshRoutes(ctx context.Context) error {
	if o.uncached {
		return ErrIndexSnapshotUnavailable
	}
	if same, err := sameIndexFiles(ctx, o.config); err != nil {
		return err
	} else if !same {
		o.uncached = true
		return ErrIndexSnapshotUnavailable
	}
	if same, err := sameIndexFiles(ctx, o.routes); err != nil {
		return err
	} else if !same {
		fresh, err := o.project.ObserveIndex(ctx)
		if err != nil {
			return err
		}
		fresh.afterIndexQuery = o.afterIndexQuery
		*o = *fresh
	}
	return nil
}

func (o *IndexObserver) checkSharedFiles(ctx context.Context, shared []indexFile, digests [][32]byte) ([]indexFile, error) {
	after, err := o.sharedFiles(ctx)
	if err != nil {
		return nil, err
	}
	if len(after) != len(shared) {
		return nil, errIndexChanged
	}
	for i, file := range after {
		old := shared[i]
		if file.path != old.path || file.stamp.Serial != old.stamp.Serial || file.stamp.FileID != old.stamp.FileID || file.stamp.Size != old.stamp.Size {
			return nil, errIndexChanged
		}
		// Git itself touches shared-index mtime on reads. Accept that only
		// after proving the bytes did not change across the real Git query.
		got, err := indexDigest(ctx, file.path)
		if err != nil {
			return nil, err
		}
		if got != digests[i] {
			return nil, errIndexChanged
		}
	}
	return after, nil
}

func (o *IndexObserver) sharedFiles(ctx context.Context) ([]indexFile, error) {
	// read-cache.c searches gitdir first, then dirname(index). Watching every
	// sharedindex.* in those locations avoids parsing Git's binary link
	// extension or launching rev-parse again when the main index changes.
	dirs := []string{o.gitdir}
	if parent := filepath.Dir(o.index); parent != o.gitdir {
		dirs = append(dirs, parent)
	}
	var files []indexFile
	for _, dir := range dirs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			prefix, _, hasDot := strings.Cut(entry.Name(), ".")
			isShared := prefix == "sharedindex"
			if hasDot && !isShared && caseInsensitive {
				isShared, err = (winfs.FS{}).SameName(prefix, "sharedindex")
				if err != nil {
					return nil, err
				}
			}
			if hasDot && isShared {
				path := filepath.Join(dir, entry.Name())
				if err := pathutil.Unchanged(path); err != nil {
					return nil, err
				}
				file, err := indexMetadata(path, false)
				if err != nil {
					return nil, err
				}
				files = append(files, file)
			}
		}
	}
	return files, nil
}

func indexDigest(ctx context.Context, path string) ([32]byte, error) {
	if err := ctx.Err(); err != nil {
		return [32]byte{}, err
	}
	f, err := openIndex(path)
	if err != nil {
		return [32]byte{}, err
	}
	defer f.Close()
	digest := sha256.New()
	buf := make([]byte, 64*1024)
	carry := 0
	for {
		if err := ctx.Err(); err != nil {
			return [32]byte{}, err
		}
		n, err := f.Read(buf[carry:])
		chunk := buf[:carry+n]
		// Git's sparse-index format requires the sdir extension. ls-files
		// --sparse may expand it in memory, hiding its tree dependencies.
		// Reject the marker anywhere, including across read boundaries. This
		// deliberately permits false positives (fresh queries), never guesses
		// membership or implements another index decoder. See Git's
		// Documentation/technical/sparse-index.adoc and read-cache.c.
		if bytes.Contains(chunk, []byte("sdir")) {
			return [32]byte{}, ErrIndexSnapshotUnavailable
		}
		_, _ = digest.Write(buf[carry : carry+n])
		if errors.Is(err, io.EOF) {
			return [32]byte(digest.Sum(nil)), nil
		}
		if err != nil {
			return [32]byte{}, fmt.Errorf("read Git index: %w", err)
		}
		carry = min(3, len(chunk))
		copy(buf[:carry], chunk[len(chunk)-carry:])
	}
}
