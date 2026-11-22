package docker

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

type VolumeArchive struct {
	Digest  string
	Bytes   int64
	Entries int
}

type archiveReceipt = VolumeArchive

func VerifyVolumeArchive(r io.Reader) (VolumeArchive, error) { return verifyVolumeArchive(r) }

type byteCount int64

func (n *byteCount) Write(p []byte) (int, error) { *n += byteCount(len(p)); return len(p), nil }

// Read the persisted archive, including its padding, without extracting any host paths.
func verifyVolumeArchive(r io.Reader) (archiveReceipt, error) {
	var n byteCount
	hash := sha256.New()
	raw := io.TeeReader(r, io.MultiWriter(hash, &n))
	tr := tar.NewReader(raw)
	tree := make(map[string]byte)
	for {
		before := n
		h, err := tr.Next()
		if err == io.EOF {
			// archive/tar also accepts a stream missing one or both end-of-archive blocks.
			padding := (-before) & 511
			if n-before != padding+1024 || tree["."] != tar.TypeDir {
				return archiveReceipt{}, errors.New("incomplete volume tar")
			}
			break
		}
		if err != nil {
			return archiveReceipt{}, err
		}
		if err := recordTarEntry(h, tree); err != nil {
			return archiveReceipt{}, err
		}
		if _, err := io.Copy(io.Discard, tr); err != nil {
			return archiveReceipt{}, err
		}
	}
	var block [512]byte
	for {
		_, err := io.ReadFull(raw, block[:])
		if err == io.EOF {
			break
		}
		if err != nil {
			return archiveReceipt{}, err
		}
		if !bytes.Equal(block[:], zeroTarBlock[:]) {
			return archiveReceipt{}, errors.New("unexpected data after volume tar footer")
		}
	}
	return archiveReceipt{hex.EncodeToString(hash.Sum(nil)), int64(n), len(tree)}, nil
}

var zeroTarBlock [512]byte

func tarEntryName(name string) (string, error) {
	name = strings.TrimPrefix(name, "./")
	if name == "" || path.IsAbs(name) || path.Clean(name) != name || name == ".." || strings.HasPrefix(name, "../") {
		return "", errors.New("noncanonical volume tar entry")
	}
	return name, nil
}

func recordTarEntry(h *tar.Header, tree map[string]byte) error {
	name := h.Name
	if h.Typeflag == tar.TypeDir {
		name = strings.TrimSuffix(name, "/")
	}
	name, err := tarEntryName(name)
	if err != nil {
		return err
	}
	if _, exists := tree[name]; exists {
		return errors.New("duplicate volume tar entry")
	}
	if name == "." && h.Typeflag != tar.TypeDir || name != "." && tree[path.Dir(name)] != tar.TypeDir {
		return errors.New("volume tar parent is not an archived directory")
	}
	switch h.Typeflag {
	case tar.TypeDir, tar.TypeReg, tar.TypeSymlink:
	case tar.TypeLink:
		target, err := tarEntryName(h.Linkname)
		if err != nil || tree[target] != tar.TypeReg {
			return errors.New("invalid volume tar hard link")
		}
	default:
		return fmt.Errorf("unsupported volume tar entry type %d", h.Typeflag)
	}
	tree[name] = h.Typeflag
	return nil
}
