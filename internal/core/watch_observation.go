package core

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"time"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"github.com/woodfishhhh/obituary/internal/pathutil"
)

const WatchStableWindow = 10 * time.Minute

func (s *Service) watchStableWindow() time.Duration {
	if s.WatchStableWindow > 0 {
		return s.WatchStableWindow
	}
	return WatchStableWindow
}

type WatchSnapshot = corefs.WatchSnapshot

// WatchTiming retains the registration's age basis even after a policy edit.
type WatchTiming struct {
	Age         string        `json:"age"`
	Lifespan    string        `json:"lifespan"`
	Automatic   time.Time     `json:"automatic"`
	StableSince time.Time     `json:"stableSince"`
	Snapshot    WatchSnapshot `json:"snapshot"`
}

func (s *Service) observeWatch(path string, want Identity, isDir, hold bool) (WatchSnapshot, func(), error) {
	return s.observeObjectTree(path, want, isDir, hold, true)
}

// First-seen directories need only their own metadata. Modified policies still
// need the newest descendant date; that scan maintains the displayed lifespan,
// and never authorizes an automatic directory movement.
func (s *Service) observeWatchChild(path string, want Identity, isDir bool, age string) (WatchSnapshot, func(), error) {
	if !isDir || age == WatchModified {
		return s.observeWatch(path, want, isDir, false)
	}
	if err := checkWatchDownload(path); err != nil {
		return WatchSnapshot{}, nil, err
	}
	return s.FS.ObserveWatch(path, false)
}

func (s *Service) observeObjectTree(path string, want Identity, isDir, hold, downloads bool) (WatchSnapshot, func(), error) {
	o := watchObserver{svc: s, root: path, hash: sha256.New(), hold: hold, downloads: downloads}
	snap, err := o.visit(path)
	if err == nil && (!sameWatchIdentity(snap.Identity, want) || snap.Directory != isDir) {
		err = fmt.Errorf("watch observation identity changed")
	}
	if err != nil {
		o.release()
		return snap, nil, err
	}
	if isDir {
		snap.Tree = fmt.Sprintf("%x", o.hash.Sum(nil))
		// Windows directory rename rejects open descendants even with shared
		// delete. Keep the tree snapshot, then release before MovePinned.
		o.release()
		o.releases = nil
	}
	return snap, o.release, nil
}

type watchObserver struct {
	svc       *Service
	root      string
	hash      hash.Hash
	hold      bool
	downloads bool
	count     int
	releases  []func()
}

func (o *watchObserver) release() {
	for i := len(o.releases) - 1; i >= 0; i-- {
		o.releases[i]()
	}
}

func (o *watchObserver) visit(path string) (WatchSnapshot, error) {
	if err := o.svc.Store.Context().Err(); err != nil {
		return WatchSnapshot{}, err
	}
	o.count++
	if o.count > 4096 {
		return WatchSnapshot{}, fmt.Errorf("folder stability observation exceeds 4096 objects; retry or spare it")
	}
	if o.downloads && watchDownload(path) {
		return WatchSnapshot{}, fmt.Errorf("download temporary name is still present: %s", path)
	}
	if o.downloads && path == o.root {
		if err := checkWatchDownload(path); err != nil {
			return WatchSnapshot{}, err
		}
	}
	snap, release, err := o.svc.FS.ObserveWatch(path, o.hold)
	if err != nil {
		return snap, err
	}
	if o.hold {
		o.releases = append(o.releases, release)
	} else {
		release()
	}
	rel, err := filepath.Rel(o.root, path)
	if err != nil {
		return snap, err
	}
	fmt.Fprintf(o.hash, "%q %v %d %d\n", rel, snap.Identity, snap.Size, snap.Modified.UnixNano())
	if !snap.Directory {
		return snap, nil
	}
	return o.children(path, snap)
}

func (o *watchObserver) children(path string, snap WatchSnapshot) (WatchSnapshot, error) {
	children, err := os.ReadDir(path)
	if err != nil {
		return snap, err
	}
	for _, child := range children {
		childPath := filepath.Join(path, child.Name())
		if err := pathutil.Unchanged(childPath); err != nil {
			return snap, err
		}
		nested, err := o.visit(childPath)
		if err != nil {
			return snap, err
		}
		if nested.Modified.After(snap.Modified) {
			snap.Modified = nested.Modified
		}
	}
	return snap, nil
}

func sameWatchSnapshot(a, b WatchSnapshot) bool {
	return sameWatchIdentity(a.Identity, b.Identity) && a.Size == b.Size && a.Modified.Equal(b.Modified) && a.Tree == b.Tree
}

func updateWatchTiming(it *Item, p WatchPolicy, snap WatchSnapshot, now time.Time) error {
	before := *it
	if it.Watch == nil {
		it.Watch = &WatchTiming{Age: p.Age, Lifespan: p.Lifespan, StableSince: now, Snapshot: snap}
	} else {
		copy := *it.Watch
		it.Watch = &copy
	}
	w := it.Watch
	if w.StableSince.IsZero() || now.Before(w.StableSince) || !sameWatchSnapshot(w.Snapshot, snap) {
		w.StableSince, w.Snapshot = now, snap
	}
	d, err := ParseDuration(w.Lifespan)
	if err != nil {
		return err
	}
	age := it.CreatedAt
	if w.Age == WatchModified {
		age = snap.Modified
	}
	if w.Age != WatchFirstSeen && w.Age != WatchModified {
		return fmt.Errorf("unknown watch age basis")
	}
	if next := age.Add(d); next.After(w.Automatic) {
		w.Automatic = next
	}
	if w.Automatic.After(it.ExpiresAt) {
		it.ExpiresAt = w.Automatic
	}
	if before.Watch == nil || *before.Watch != *w || !before.ExpiresAt.Equal(it.ExpiresAt) {
		it.Rev++
		it.Attempts, it.NextAttempt, it.LastError = 0, time.Time{}, ""
	}
	return nil
}

// The caller commits timing changes even when the move must wait.
func (s *Service) holdWatchForMove(st *State, it Item, path, reason string) (func(), error) {
	noop := func() {}
	if !s.watchMove(it, reason) {
		return noop, nil
	}
	if it.IsDir {
		if s.approvedWatch == nil {
			return noop, fmt.Errorf("directory requires burial approval")
		}
		_, err := watchBurialItem(st, *s.approvedWatch)
		return noop, err
	}
	if err := s.checkWatchScope(st, it, path, reason); err != nil {
		return noop, err
	}
	idx := st.Index(it.ID)
	if idx < 0 {
		return noop, errClaimChanged
	}
	p := watchByID(st, it.WatchID)
	snap, release, err := s.observeWatch(path, *it.Identity, it.IsDir, true)
	if err != nil {
		return noop, err
	}
	if err = updateWatchTiming(&st.Items[idx], p, snap, s.now()); err == nil {
		current := st.Items[idx]
		if s.now().Sub(current.Watch.StableSince) < s.watchStableWindow() || current.ExpiresAt.After(s.now()) {
			status := s.WatchStatus(current, s.now())
			err = fmt.Errorf("folder child is still active; quiet for %s more (stable_window=%s); stable at %s", status.QuietFor, status.StableWindow, status.StableAt.Format(time.RFC3339))
		}
	}
	if err == nil {
		err = s.checkWatchScope(st, it, path, reason)
	}
	if err != nil {
		release()
		return noop, err
	}
	return release, nil
}
