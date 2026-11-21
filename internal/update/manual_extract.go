package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const programLimit = 128 << 20

// extractPrograms writes the named top-level files of a verified release
// archive into dest. Entries with any other name, including every path with a
// directory part, are ignored, so an archive cannot place a file elsewhere.
func extractPrograms(data []byte, archive string, names []string, dest string) error {
	written := map[string]bool{}
	var err error
	if strings.HasSuffix(archive, ".zip") {
		err = eachZip(data, func(name string, r io.Reader) error { return keepProgram(names, written, dest, name, r) })
	} else {
		err = eachTar(data, func(name string, r io.Reader) error { return keepProgram(names, written, dest, name, r) })
	}
	if err != nil {
		return err
	}
	for _, name := range names {
		if !written[name] {
			return fmt.Errorf("release archive is missing %s; refusing to install", name)
		}
	}
	return nil
}

func keepProgram(names []string, written map[string]bool, dest, name string, r io.Reader) error {
	name = strings.TrimPrefix(name, "./")
	if !slices.Contains(names, name) {
		return nil
	}
	if written[name] {
		return fmt.Errorf("release archive lists %s twice; refusing to install", name)
	}
	f, err := os.OpenFile(filepath.Join(dest, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(r, programLimit+1))
	if err = errors.Join(err, f.Close()); err == nil && n > programLimit {
		err = fmt.Errorf("%s is larger than %d bytes", name, programLimit)
	}
	written[name] = err == nil
	return err
}

func eachZip(data []byte, visit func(name string, r io.Reader) error) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	// An odd path is not an error here: only exact top-level names are used.
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return fmt.Errorf("read release archive: %w", err)
	}
	for _, f := range zr.File {
		if !f.FileInfo().Mode().IsRegular() {
			continue
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		err = errors.Join(visit(f.Name, r), r.Close())
		if err != nil {
			return err
		}
	}
	return nil
}

func eachTar(data []byte, visit func(name string, r io.Reader) error) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("read release archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read release archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if err := visit(header.Name, tr); err != nil {
			return err
		}
	}
}
