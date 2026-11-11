package winfs

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMetadataDistinguishesReplacementWithSameSizeAndTime(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "index")
	if err := os.WriteFile(path, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := ReadMetadata(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".lock", []byte("other"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path+".lock", info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path+".lock", path); err != nil {
		t.Fatal(err)
	}
	after, err := ReadMetadata(path)
	if err != nil || before.Serial != after.Serial || before.FileID == after.FileID || before.Size != after.Size || before.Modified != after.Modified {
		t.Fatalf("metadata missed equal-size equal-time replacement: %+v %+v %v", before, after, err)
	}
}

func TestMetadataByNameMatchesHandle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	file := filepath.Join(dir, "index")
	if err := os.WriteFile(file, []byte("contents"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{file, dir} {
		fast, supported, err := metadataByName(path)
		if err != nil {
			t.Fatal(err)
		}
		if !supported {
			t.Skip("GetFileInformationByName basic information is unavailable")
		}
		want, err := metadataByHandle(path)
		if err != nil || fast != want {
			t.Fatalf("by-name metadata differs from pinned handle: %+v %+v %v", fast, want, err)
		}
	}
}

func TestMetadataFollowsJunction(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	base := t.TempDir()
	real, link := filepath.Join(base, "real"), filepath.Join(base, "link")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, real).CombinedOutput(); err != nil {
		t.Fatalf("junction: %s %v", out, err)
	}
	got, err := ReadMetadata(link)
	want, wantErr := metadataByHandle(real)
	if err != nil || wantErr != nil || got != want {
		t.Fatalf("metadata observed reparse object instead of target: %+v %+v %v %v", got, want, err, wantErr)
	}
}

func BenchmarkReadMetadata(b *testing.B) {
	path := filepath.Join(b.TempDir(), "index")
	if err := os.WriteFile(path, []byte("contents"), 0600); err != nil {
		b.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		read func(string) (MetadataStamp, error)
	}{{"handle", metadataByHandle}, {"selected", ReadMetadata}} {
		b.Run(tc.name, func(b *testing.B) {
			for b.Loop() {
				if _, err := tc.read(path); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
