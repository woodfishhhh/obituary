package update

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractProgramsFromBothArchiveKinds(t *testing.T) {
	for _, archive := range []string{"obituary_1.4.4_windows_amd64.zip", "obituary_1.4.4_linux_amd64.tar.gz"} {
		t.Run(archive, func(t *testing.T) {
			names := []string{"obit", "obitd"}
			data := packArchive(t, archive, map[string][]byte{
				"obit": []byte("one"), "obitd": []byte("two"), "LICENSE": []byte("license"),
				"../escape": []byte("x"), "sub/obit": []byte("nested"), "./obit-gui": []byte("not asked for"),
			})
			dest := t.TempDir()
			if err := extractPrograms(data, archive, names, dest); err != nil {
				t.Fatal(err)
			}
			entries, _ := os.ReadDir(dest)
			if len(entries) != 2 {
				t.Fatalf("extracted %d files", len(entries))
			}
			for name, want := range map[string]string{"obit": "one", "obitd": "two"} {
				if got, err := os.ReadFile(filepath.Join(dest, name)); err != nil || string(got) != want {
					t.Errorf("%s: %q %v", name, got, err)
				}
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(dest), "escape")); err == nil {
				t.Error("an entry escaped the destination")
			}
		})
	}
}

func TestExtractProgramsRejectsBadArchives(t *testing.T) {
	names := []string{"obit", "obitd"}
	for _, archive := range []string{"x.zip", "x.tar.gz"} {
		for name, tc := range map[string]struct {
			files map[string][]byte
			want  string
		}{
			"missing program": {map[string][]byte{"obit": []byte("one")}, "missing obitd"},
			"empty":           {map[string][]byte{}, "missing obit"},
		} {
			t.Run(archive+" "+name, func(t *testing.T) {
				err := extractPrograms(packArchive(t, archive, tc.files), archive, names, t.TempDir())
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("err=%v", err)
				}
			})
		}
		t.Run(archive+" not an archive", func(t *testing.T) {
			if err := extractPrograms([]byte("not an archive"), archive, names, t.TempDir()); err == nil || !strings.Contains(err.Error(), "read release archive") {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestExtractProgramsRefusesToOverwrite(t *testing.T) {
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "obit"), []byte("already here"), 0o755); err != nil {
		t.Fatal(err)
	}
	archive := "x.tar.gz"
	err := extractPrograms(packArchive(t, archive, map[string][]byte{"obit": []byte("one")}), archive, []string{"obit"}, dest)
	if err == nil {
		t.Fatal("an existing file was overwritten")
	}
	if got, _ := os.ReadFile(filepath.Join(dest, "obit")); string(got) != "already here" {
		t.Fatalf("content %q", got)
	}
}
