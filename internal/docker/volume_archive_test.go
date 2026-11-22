package docker

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestVolumeArchiveVerifiesCompleteSafeTree(t *testing.T) {
	root := tar.Header{Name: "./", Typeflag: tar.TypeDir, Mode: 0700}
	file := tar.Header{Name: "./a", Mode: 0640, Size: 3, Typeflag: tar.TypeReg}
	link := tar.Header{Name: "./b", Typeflag: tar.TypeSymlink, Linkname: "../outside"}
	build := func(headers ...tar.Header) []byte {
		t.Helper()
		var b bytes.Buffer
		w := tar.NewWriter(&b)
		for _, h := range headers {
			if err := w.WriteHeader(&h); err != nil {
				t.Fatal(err)
			}
			if h.Size > 0 {
				if _, err := w.Write([]byte{'a', 0, 'b'}); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	valid := build(root, file, link)
	sum := fmt.Sprintf("%x", sha256.Sum256(valid))
	got, err := verifyVolumeArchive(bytes.NewReader(valid))
	if err != nil || got.Digest != sum || got.Bytes != int64(len(valid)) || got.Entries != 3 {
		t.Fatal(got, err)
	}
	for name, data := range map[string][]byte{"root-missing": build(file), "truncated-footer": valid[:len(valid)-512], "foreign-tail": append(bytes.Clone(valid), 'a'), "duplicate": build(root, file, file), "write-via-link": build(root, link, tar.Header{Name: "./b/x", Typeflag: tar.TypeReg}), "file-before-link-parent": build(root, tar.Header{Name: "./b/x", Typeflag: tar.TypeReg}, link), "device": build(root, tar.Header{Name: "./c", Typeflag: tar.TypeChar}), "escape": build(root, tar.Header{Name: "../../x", Typeflag: tar.TypeReg}), "external-hardlink": build(root, tar.Header{Name: "./h", Typeflag: tar.TypeLink, Linkname: "../kept"})} {
		t.Run(name, func(t *testing.T) {
			if _, err := verifyVolumeArchive(bytes.NewReader(data)); err == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
}
