package lifecycle

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

func readBurialManifest(path string, p *PendingGrave) (m burialManifest, err error) {
	info, err := os.Lstat(path)
	if err != nil {
		return m, err
	}
	if !info.Mode().IsRegular() || info.Size() > manifestBytes {
		return m, errors.New("manifest is not a regular JSON file of at most 1 MiB")
	}
	data, err := readShared(path)
	if err != nil {
		return m, err
	}
	p.partial = &m.Burial
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return m, err
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return m, errors.New("manifest must contain exactly one JSON object")
	}
	return m, nil
}

func validateManifestMetadata(m burialManifest, id int64) ([]*Identity, error) {
	b := m.Burial
	validVersion := m.Version == 1 && m.Resurrect == nil || m.Version == 2 && m.Resurrect != nil ||
		m.Version == 3 && m.Resurrect == nil && m.WatchCheck != nil && b.Source == "watch" ||
		m.Version == 4 && m.Resurrect == nil && b.IdentityMismatch != ""
	if m.Version != 3 && (m.WatchCheck != nil || m.WatchError != "") {
		validVersion = false
	}
	if !validVersion || b.ID < 1 || b.ID != id || b.Identity == nil || b.GraveID == nil ||
		b.BuriedAt.IsZero() || !b.PurgeAt.After(b.BuriedAt) || b.Size != 0 ||
		(b.Reason != ReasonBury && b.Reason != ReasonExpired) {
		return nil, errors.New("invalid manifest version, ID, identities, time or reason")
	}
	seen := map[int64]bool{b.ID: true}
	identities := []*Identity{b.Identity, b.GraveID}
	for _, child := range b.Children {
		if !b.IsDir || child.ID < 1 || seen[child.ID] || child.Identity == nil {
			return nil, errors.New("invalid contained identity snapshot")
		}
		seen[child.ID] = true
		identities = append(identities, child.Identity)
	}
	seen = map[int64]bool{b.ID: true}
	for _, child := range b.Contains {
		if !b.IsDir || child < 1 || seen[child] || !slices.ContainsFunc(b.Children, func(it BuriedChild) bool { return it.ID == child }) {
			return nil, errors.New("invalid contained lifespan IDs")
		}
		seen[child] = true
	}
	return identities, nil
}

func validateManifestName(name string, b Burial) error {
	wantName := fmt.Sprintf("%d-%s-", b.ID, b.BuriedAt.UTC().Format("20060102T150405Z"))
	suffix, hexErr := hex.DecodeString(strings.TrimPrefix(name, wantName))
	if !strings.HasPrefix(name, wantName) || hexErr != nil || len(suffix) != 3 {
		return errors.New("manifest name does not match its burial ID and time")
	}
	if !filepath.IsAbs(b.Path) || !filepath.IsAbs(b.Grave) ||
		!samePath(filepath.Base(filepath.Dir(b.Grave)), name) || !samePath(filepath.Base(b.Grave), filepath.Base(b.Path)) {
		return errors.New("manifest paths do not match its grave name")
	}
	return nil
}

func (s *Service) validateManifestRoot(root string, b Burial) error {
	realRoot, err := s.FS.RealPath(root)
	if err != nil {
		return err
	}
	parent, err := s.FS.RealPath(filepath.Dir(filepath.Dir(b.Grave)))
	if err != nil {
		return err
	}
	expected, err := s.FS.RealPath(s.graveyardRoot(b.Path))
	if err != nil {
		return err
	}
	parentOK, parentErr := s.sameDirectory(parent, realRoot)
	expectedOK, expectedErr := s.sameDirectory(expected, realRoot)
	if err := errors.Join(parentErr, expectedErr); err != nil {
		return err
	}
	if !parentOK || !expectedOK {
		return errors.New("manifest grave escapes this ledger's graveyard root")
	}
	if samePath(b.Path, root) || isInside(b.Path, root) || samePath(b.Path, realRoot) || isInside(b.Path, realRoot) {
		return errors.New("manifest original path is inside the graveyard")
	}
	// The source may already be gone, but an existing parent must not resolve
	// back into the graveyard through an alias either.
	if originalParent, err := s.FS.RealPath(filepath.Dir(b.Path)); err == nil &&
		(samePath(originalParent, realRoot) || isInside(originalParent, realRoot)) {
		return errors.New("manifest original parent resolves into the graveyard")
	}
	return nil
}

func validateManifestIdentities(b Burial, identities []*Identity) error {
	grave, err := pathutil.Abs(b.Grave)
	if err != nil {
		return err
	}
	for _, id := range identities {
		*id = id.Normalized()
		raw, err := hex.DecodeString(id.FileID)
		volume, volumeErr := pathutil.Abs(id.Volume)
		if err != nil || volumeErr != nil || len(raw) != 16 || !samePath(pathutil.VolumeRoot(volume), pathutil.VolumeRoot(grave)) {
			return errors.New("invalid manifest object identity")
		}
	}
	return nil
}
