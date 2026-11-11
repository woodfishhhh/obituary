package core

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// ProposalProjectDeleted requires a missing root on an accessible volume.
// Every existing ancestor must be ordinary: a link higher up can hide a mount.
func (s *Service) ProposalProjectDeleted(root string) (bool, error) {
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if !filepath.IsAbs(root) {
		return false, fmt.Errorf("project path is not absolute")
	}
	if err := pathutil.Unchanged(root); err != nil {
		return false, err
	}
	volume := pathutil.VolumeRoot(root)
	volumeID, err := s.FS.Identify(volume)
	if err != nil {
		return false, fmt.Errorf("volume unavailable: %w", err)
	}
	if err := readableProposalDirectory(volume); err != nil {
		return false, err
	}
	nearest := ""
	for dir := filepath.Dir(root); ; dir = filepath.Dir(dir) {
		info, err := os.Lstat(dir)
		if errors.Is(err, os.ErrNotExist) && nearest == "" && dir != filepath.Dir(dir) {
			continue
		}
		if err != nil {
			return false, err
		}
		if !info.IsDir() || isReparse(info) {
			return false, fmt.Errorf("ancestor %q is not an ordinary directory", dir)
		}
		if nearest == "" {
			nearest = dir
		}
		if dir == filepath.Dir(dir) {
			break
		}
	}
	ancestorID, err := s.FS.Identify(nearest)
	if err != nil {
		return false, err
	}
	if ancestorID.Serial != volumeID.Serial {
		return false, fmt.Errorf("ancestor %q is on a different volume", nearest)
	}
	if err := readableProposalDirectory(nearest); err != nil {
		return false, err
	}
	// A recreated root must be reconsidered by the ordinary Git checks.
	_, err = os.Lstat(root)
	return errors.Is(err, os.ErrNotExist), nonAbsenceError(err)
}

func readableProposalDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || isReparse(info) {
		return fmt.Errorf("directory %q is not ordinary", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	// Reading one entry verifies enumeration permission without scanning a tree.
	_, err = f.Readdirnames(1)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func nonAbsenceError(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
