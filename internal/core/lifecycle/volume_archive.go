package lifecycle

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/docker"
	"github.com/woodfishhhh/obituary/internal/pathutil"
)

type volumeArchiveHandle struct {
	root *os.Root
	sub  *os.Root
	dir  string
}

func (h *volumeArchiveHandle) Close() {
	if h == nil {
		return
	}
	if h.sub != nil {
		_ = h.sub.Close()
	}
	if h.root != nil {
		_ = h.root.Close()
	}
}

func hasDotDotSegment(path string) bool {
	for _, part := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return true
		}
	}
	return false
}

func validVolumeArchiveDir(id int64, dir string) bool {
	if id < 1 || !filepath.IsLocal(dir) || strings.ContainsAny(dir, `/\`) {
		return false
	}
	suffix, ok := strings.CutPrefix(dir, fmt.Sprintf("%d-", id))
	if !ok || len(suffix) != 16 {
		return false
	}
	_, err := hex.DecodeString(suffix)
	return err == nil
}

func (s *Service) sameArchivePath(a, b string) (bool, error) {
	if s.FS != nil {
		return s.FS.SameName(a, b)
	}
	return samePath(a, b), nil
}

func (s *Service) resolveVolumeArchiveDir(b Burial) (string, error) {
	if b.ID < 1 || b.Grave == "" || hasDotDotSegment(b.Grave) || filepath.Base(b.Grave) != "volume.tar" {
		return "", fmt.Errorf("#%d: %s is not a Docker volume archive in this graveyard: %w", b.ID, b.Grave, ErrUnsafe)
	}
	absRoot, err := pathutil.Abs(filepath.Join(s.Store.Home, "graveyard", "docker-volumes"))
	if err != nil {
		return "", err
	}
	absGrave, err := pathutil.Abs(b.Grave)
	if err != nil {
		return "", err
	}
	dir := filepath.Base(filepath.Dir(absGrave))
	if filepath.Base(absGrave) != "volume.tar" || !validVolumeArchiveDir(b.ID, dir) {
		return "", fmt.Errorf("#%d: %s is not a Docker volume archive in this graveyard: %w", b.ID, b.Grave, ErrUnsafe)
	}
	expected := filepath.Join(absRoot, dir, "volume.tar")
	same, err := s.sameArchivePath(absGrave, expected)
	if err != nil {
		return "", err
	}
	if !same {
		return "", fmt.Errorf("#%d: %s is not a Docker volume archive in this graveyard: %w", b.ID, b.Grave, ErrUnsafe)
	}
	return dir, nil
}

func (s *Service) openVolumeArchiveRoot(id int64, allowMissing bool) (*os.Root, error) {
	homeRoot, err := os.OpenRoot(s.Store.Home)
	if allowMissing && errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer homeRoot.Close()
	for _, rel := range []string{"graveyard", filepath.Join("graveyard", "docker-volumes")} {
		info, statErr := homeRoot.Lstat(rel)
		if allowMissing && errors.Is(statErr, os.ErrNotExist) {
			return nil, nil
		}
		if statErr != nil {
			return nil, statErr
		}
		if !info.IsDir() || isLink(info) {
			return nil, fmt.Errorf("#%d: Docker volume graveyard %s is not a plain directory: %w", id, filepath.Join(s.Store.Home, rel), ErrUnsafe)
		}
	}
	return homeRoot.OpenRoot(filepath.Join("graveyard", "docker-volumes"))
}

func (s *Service) openVolumeArchive(b Burial, allowMissingDir bool) (*volumeArchiveHandle, error) {
	dir, err := s.resolveVolumeArchiveDir(b)
	if err != nil {
		return nil, err
	}
	root, err := s.openVolumeArchiveRoot(b.ID, allowMissingDir)
	if err != nil {
		return nil, err
	}
	if root == nil {
		return &volumeArchiveHandle{dir: dir}, nil
	}
	info, err := root.Lstat(dir)
	if allowMissingDir && errors.Is(err, os.ErrNotExist) {
		_ = root.Close()
		return &volumeArchiveHandle{dir: dir}, nil
	}
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	if !info.IsDir() || isLink(info) {
		_ = root.Close()
		return nil, fmt.Errorf("#%d: Docker volume archive directory is not a plain directory: %w", b.ID, ErrUnsafe)
	}
	sub, err := root.OpenRoot(dir)
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	return &volumeArchiveHandle{root: root, sub: sub, dir: dir}, nil
}

func openRegularInRoot(root *os.Root, name string) (*os.File, fs.FileInfo, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() || isLink(info) {
		return nil, nil, fmt.Errorf("%s is not a regular file: %w", name, ErrUnsafe)
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	if !fi.Mode().IsRegular() || isLink(fi) {
		_ = f.Close()
		return nil, nil, fmt.Errorf("%s is not a regular file: %w", name, ErrUnsafe)
	}
	return f, fi, nil
}

func removeVolumeArchive(h *volumeArchiveHandle) error {
	if h == nil {
		return nil
	}
	if h.sub != nil {
		if err := h.sub.Remove("volume.tar"); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		_ = h.sub.Remove("volume.tar.json")
		_ = h.sub.Close()
		h.sub = nil
	}
	if h.root != nil {
		_ = h.root.Remove(h.dir)
	}
	return nil
}

func verifyVolumePurgeArchive(h *volumeArchiveHandle, b Burial) error {
	if err := checkVolumePurgeManifest(h.sub, b); err != nil {
		return err
	}
	if h.sub == nil {
		return nil
	}
	f, _, err := openRegularInRoot(h.sub, "volume.tar")
	if b.PurgeIntent != nil && errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("refusing to purge unverified Docker volume archive: %w", err)
	}
	defer f.Close()
	check, err := docker.VerifyVolumeArchive(f)
	if err != nil {
		return fmt.Errorf("refusing to purge unverified Docker volume archive: %w", err)
	}
	if check.Digest != b.VolumeArchiveDigest {
		return errors.New("refusing to purge unverified Docker volume archive: archive digest mismatch")
	}
	return nil
}

func checkVolumePurgeManifest(sub *os.Root, b Burial) error {
	if sub == nil {
		return nil
	}
	f, info, err := openRegularInRoot(sub, "volume.tar.json")
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("refusing to purge Docker volume archive with unreadable evidence: %w", err)
	}
	defer f.Close()
	if info.Size() > manifestBytes {
		return errors.New("refusing to purge Docker volume archive with invalid evidence: manifest exceeds 1 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(f, manifestBytes+1))
	if err != nil {
		return fmt.Errorf("refusing to purge Docker volume archive with unreadable evidence: %w", err)
	}
	if len(data) > manifestBytes {
		return errors.New("refusing to purge Docker volume archive with invalid evidence: manifest exceeds 1 MiB")
	}
	var m burialManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("refusing to purge Docker volume archive with invalid evidence: %w", err)
	}
	if m.Version != 1 {
		return fmt.Errorf("refusing to purge Docker volume archive with invalid evidence: unsupported manifest version %d", m.Version)
	}
	if m.VolumePhase != "removed" {
		return fmt.Errorf("refusing to purge Docker volume archive with incomplete evidence (%s)", m.VolumePhase)
	}
	if m.ID != b.ID || m.Kind != KindDockerVolume || !samePath(m.Grave, b.Grave) || m.VolumeArchiveDigest != b.VolumeArchiveDigest {
		return errors.New("refusing to purge Docker volume archive with mismatched evidence")
	}
	return nil
}
