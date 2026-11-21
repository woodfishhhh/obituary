package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
)

// fakeProgram is what a fake release program contains: a marker line, so
// fakeRun can read it without starting anything.
func fakeProgram(behavior string) []byte {
	return []byte("fake program" + fakeMarker + behavior + "\n")
}

var realBinary = sync.OnceValues(func() ([]byte, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(exe)
})

// realProgram is a runnable copy of the test binary that answers --version.
func realProgram(t *testing.T, behavior string) []byte {
	t.Helper()
	data, err := realBinary()
	if err != nil {
		t.Fatal(err)
	}
	return append(append([]byte{}, data...), fakeProgram(behavior)...)
}

// fakeRun answers "<program> --version" from the program file's marker line,
// like a process would, and fails any other command.
func fakeRun(t *testing.T) runner {
	return func(_ context.Context, out io.Writer, argv []string) error {
		if len(argv) != 2 || argv[1] != "--version" {
			t.Errorf("unexpected command %q", argv)
			return fmt.Errorf("unexpected command")
		}
		data, err := os.ReadFile(argv[0])
		if err != nil {
			return err
		}
		behavior, ok := trailerBehavior(data)
		if !ok || behavior == "crash" {
			return fmt.Errorf("%s did not start", argv[0])
		}
		if version, ok := strings.CutPrefix(behavior, "version="); ok {
			fmt.Fprintln(out, "obit version", version)
		}
		return nil
	}
}

// release describes one fake release and how its server misbehaves.
type release struct {
	version  string
	files    map[string][]byte // archive entries; nil means every program, answering version
	checksum string            // overrides the archive's hash in checksums.txt
	corrupt  bool              // serve archive bytes that differ from the checksum
	noEntry  bool              // checksums.txt without the archive's line
	status   map[string]int    // path suffix -> HTTP status
}

func (r release) archive(t *testing.T, program func(name string) []byte) (string, []byte) {
	t.Helper()
	name, ok := archiveName(r.version, runtime.GOOS, runtime.GOARCH)
	if !ok {
		t.Skipf("no release archive for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	files := r.files
	if files == nil {
		files = map[string][]byte{"LICENSE": []byte("license"), "README.txt": []byte("readme")}
		for _, p := range programs(runtime.GOOS, runtime.GOARCH) {
			files[p] = program(p)
		}
	}
	return name, packArchive(t, name, files)
}

func packArchive(t *testing.T, name string, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	if strings.HasSuffix(name, ".zip") {
		w := zip.NewWriter(&buf)
		for _, n := range sortedKeys(files) {
			f, err := w.CreateHeader(&zip.FileHeader{Name: n, Method: zip.Store})
			if err != nil {
				t.Fatal(err)
			}
			f.Write(files[n])
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	gz := gzip.NewWriter(&buf)
	w := tar.NewWriter(gz)
	for _, n := range sortedKeys(files) {
		w.WriteHeader(&tar.Header{Name: n, Mode: 0o755, Size: int64(len(files[n])), Typeflag: tar.TypeReg})
		w.Write(files[n])
	}
	w.Close()
	gz.Close()
	return buf.Bytes()
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// serve starts a release server and returns sources that point at it for both
// GitHub and the mirror, and the directory URL.
func (r release) serve(t *testing.T, program func(name string) []byte) (sources, string) {
	t.Helper()
	name, data := r.archive(t, program)
	sum := sha256.Sum256(data)
	listed := hex.EncodeToString(sum[:])
	if r.checksum != "" {
		listed = r.checksum
	}
	body := fmt.Sprintf("%s  %s\n%s  other.tar.gz\n", listed, name, strings.Repeat("0", 64))
	if r.noEntry {
		body = strings.Repeat("0", 64) + "  other.tar.gz\n"
	}
	served := data
	if r.corrupt {
		served = append(append([]byte{}, data[:len(data)/2]...), bytes.Repeat([]byte{'x'}, len(data)-len(data)/2)...)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		for suffix, status := range r.status {
			if strings.HasSuffix(req.URL.Path, suffix) {
				http.Error(w, "refused", status)
				return
			}
		}
		switch {
		case strings.HasSuffix(req.URL.Path, "/checksums.txt"):
			io.WriteString(w, body)
		case strings.HasSuffix(req.URL.Path, "/"+name):
			w.Write(served)
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(srv.Close)
	dir := srv.URL + "/v" + r.version + "/"
	return sources{github: dir, mirror: dir, client: srv.Client()}, dir
}

// installFixture is a shared bin folder: this release's programs next to files
// that are not Obituary's, which an update must leave exactly as they are.
type installFixture struct {
	dir, exe string
	others   map[string][]byte // relative path -> content of every file that is not a program
}

func newInstall(t *testing.T, behavior string) installFixture {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "bin")
	f := installFixture{dir: dir, others: map[string][]byte{
		"jq.exe": []byte("another tool"), "hermes.cmd": []byte("@echo hi"), "LICENSE": []byte("old license"),
		"notes.txt": []byte("mine"), filepath.Join("backup-1.4.2", "obit.exe"): []byte("an older copy"),
	}}
	for name, data := range f.others {
		write(t, filepath.Join(dir, name), data)
	}
	for _, p := range programs(runtime.GOOS, runtime.GOARCH) {
		write(t, filepath.Join(dir, p), fakeProgram(behavior))
	}
	f.exe = filepath.Join(dir, programs(runtime.GOOS, runtime.GOARCH)[0])
	return f
}

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}
}

// tree lists every file under dir (relative, slash-separated) with its hash.
func tree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		sum := sha256.Sum256(data)
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// assertOthersUntouched checks that every non-program file is byte-identical.
func (f installFixture) assertOthersUntouched(t *testing.T) {
	t.Helper()
	got := tree(t, f.dir)
	for name, want := range f.others {
		sum := sha256.Sum256(want)
		if got[filepath.ToSlash(name)] != hex.EncodeToString(sum[:]) {
			t.Errorf("%s changed or is missing", name)
		}
	}
}

// assertNoStray fails on any entry that is neither a program nor one of the
// fixture's own files nor inside an update folder.
func (f installFixture) assertNoStray(t *testing.T) {
	t.Helper()
	known := map[string]bool{}
	for name := range f.others {
		known[filepath.ToSlash(name)] = true
	}
	for _, p := range programs(runtime.GOOS, runtime.GOARCH) {
		known[p] = true
	}
	for name := range tree(t, f.dir) {
		if !known[name] && !strings.HasPrefix(name, ".obituary-update-") {
			t.Errorf("unexpected file %s", name)
		}
	}
}

// updateFolders lists the transaction folders left in the install folder.
func (f installFixture) updateFolders(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), ".obituary-update-") {
			out = append(out, filepath.Join(f.dir, e.Name()))
		}
	}
	return out
}

// programVersions reads each program's marker behavior ("version=1.4.3").
func (f installFixture) programVersions(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, p := range programs(runtime.GOOS, runtime.GOARCH) {
		data, err := os.ReadFile(filepath.Join(f.dir, p))
		if err != nil {
			t.Fatal(err)
		}
		out[p], _ = trailerBehavior(data)
	}
	return out
}

// closedServerURL is a loopback address nothing listens on: connecting fails
// at once, which is how an unreachable host looks to the updater.
func closedServerURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	return url
}
