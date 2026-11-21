package update

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// recorder is a set of ApplyHooks that notes the order they run in.
type recorder struct {
	steps   []string
	retired []string
	onReady func()
	onWait  func()
	failAt  map[string]error
}

func (r *recorder) step(name string) error {
	r.steps = append(r.steps, name)
	return r.failAt[name]
}

func (r *recorder) hooks() ApplyHooks {
	return ApplyHooks{
		Ready: func(ApplyResult) error {
			if r.onReady != nil {
				r.onReady()
			}
			return r.step("ack")
		},
		Wait: func() error {
			if r.onWait != nil {
				r.onWait()
			}
			return r.step("wait")
		},
		Stop:     func() error { return r.step("stop") },
		Restart:  func(string) error { return r.step("restart") },
		Relaunch: func(string) error { return r.step("relaunch") },
		Retire: func(paths []string) error {
			r.retired = append(r.retired, paths...)
			return r.step("retire")
		},
	}
}

func (r *recorder) order() string { return strings.Join(r.steps, " ") }

func fixedSources(src sources) func(string) sources { return func(string) sources { return src } }

func newProgramsAre(version string) func(string) []byte {
	return func(string) []byte { return fakeProgram("version=" + version) }
}

func wantVersions(t *testing.T, f installFixture, version string) {
	t.Helper()
	for name, got := range f.programVersions(t) {
		if got != "version="+version {
			t.Errorf("%s answers %q, want version %s", name, got, version)
		}
	}
}

func TestManualUpdateReplacesOnlyReleasePrograms(t *testing.T) {
	f := newInstall(t, "version=1.4.3")
	home := t.TempDir()
	src, _ := release{version: "1.4.4"}.serve(t, newProgramsAre("1.4.4"))
	rec := &recorder{}
	before := tree(t, f.dir)
	// A window must never read the previous run's outcome as this one's.
	(&tracker{home: home}).set(Progress{Phase: PhaseFailed, Error: "last time"})
	rec.onReady = func() {
		if p, err := ReadProgress(home); err != nil || p != (Progress{Phase: PhaseStarting}) {
			t.Errorf("progress at the handoff: %+v, %v", p, err)
		}
	}
	rec.onWait = func() {
		// The window may close now: everything is downloaded and verified, and
		// not one file has moved.
		if p, err := ReadProgress(home); err != nil || p.Phase != PhaseReady {
			t.Errorf("progress at wait: %+v, %v", p, err)
		}
		for name, hash := range tree(t, f.dir) {
			if !strings.HasPrefix(name, ".obituary-update-") && before[name] != hash {
				t.Errorf("%s changed before the window closed", name)
			}
		}
	}
	res, err := applyFrom(t.Context(), home, f.exe, "1.4.3", "1.4.4", rec.hooks(), fakeRun(t), fixedSources(src))
	if err != nil || !res.Success || res.Version != "1.4.4" {
		t.Fatalf("result=%+v err=%v", res, err)
	}
	if rec.order() != "ack wait stop retire restart relaunch" {
		t.Fatalf("steps: %s", rec.order())
	}
	wantVersions(t, f, "1.4.4")
	f.assertOthersUntouched(t)
	f.assertNoStray(t)
	folders := f.updateFolders(t)
	if len(folders) != 1 || len(rec.retired) != 1 || filepath.Base(rec.retired[0]) != filepath.Base(folders[0]) {
		t.Fatalf("update folders %v, retired %v", folders, rec.retired)
	}
	// The previous programs wait in old/ for their lifespan to end.
	for _, p := range programs(runtime.GOOS, runtime.GOARCH) {
		data, err := os.ReadFile(filepath.Join(folders[0], "old", p))
		if err != nil || string(data) != string(fakeProgram("version=1.4.3")) {
			t.Errorf("old %s: %q, %v", p, data, err)
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(folders[0], "new")); len(entries) != 0 {
		t.Errorf("new/ still holds %d files", len(entries))
	}
	if p, _ := ReadProgress(home); p.Phase != PhaseDone {
		t.Errorf("final progress %+v", p)
	}
	if last, err := ReadLastApply(home); err != nil || last == nil || !last.Success || last.Version != "1.4.4" {
		t.Errorf("last apply %+v, %v", last, err)
	}
}

// With real processes: the new obit answers --version from a copy of the test binary.
func TestManualUpdateStartsTheNewProgram(t *testing.T) {
	if testing.Short() {
		t.Skip("starts the new program as a process; TestManualUpdateReplacesOnlyReleasePrograms covers the same steps")
	}
	dir := filepath.Join(t.TempDir(), "bin")
	f := installFixture{dir: dir}
	for _, p := range programs(runtime.GOOS, runtime.GOARCH) {
		write(t, filepath.Join(dir, p), realProgram(t, "version=1.4.3"))
	}
	f.exe = filepath.Join(dir, programs(runtime.GOOS, runtime.GOARCH)[0])
	src, _ := release{version: "1.4.4"}.serve(t, func(string) []byte { return realProgram(t, "version=1.4.4") })
	rec := &recorder{}
	res, err := applyFrom(t.Context(), t.TempDir(), f.exe, "1.4.3", "1.4.4", rec.hooks(), runCommand, fixedSources(src))
	if err != nil || !res.Success || res.Version != "1.4.4" {
		t.Fatalf("result=%+v err=%v", res, err)
	}
	wantVersions(t, f, "1.4.4")

	// A new program that cannot start is rolled back by the same code.
	src, _ = release{version: "1.4.5"}.serve(t, func(string) []byte { return realProgram(t, "crash") })
	res, err = applyFrom(t.Context(), t.TempDir(), f.exe, "1.4.4", "1.4.5", (&recorder{}).hooks(), runCommand, fixedSources(src))
	if err == nil || res.Success {
		t.Fatalf("a crashing program was accepted: %+v %v", res, err)
	}
	wantVersions(t, f, "1.4.4")
}

type failure struct {
	name   string
	setup  func(t *testing.T, rel *release, rec *recorder)
	err    string
	steps  string // hooks that ran
	folder bool   // an update folder is left (and retired)
}

func TestManualUpdateFailuresLeaveTheInstallUntouched(t *testing.T) {
	zeros := strings.Repeat("0", 64)
	for _, tc := range []failure{
		{"corrupt download", func(_ *testing.T, r *release, _ *recorder) { r.corrupt = true }, "SHA-256 mismatch", "ack restart relaunch", false},
		{"wrong checksum", func(_ *testing.T, r *release, _ *recorder) { r.checksum = zeros }, "SHA-256 mismatch", "ack restart relaunch", false},
		{"no checksum entry", func(_ *testing.T, r *release, _ *recorder) { r.noEntry = true }, "exactly one SHA-256 entry", "ack restart relaunch", false},
		{"archive missing a program", func(_ *testing.T, r *release, _ *recorder) {
			r.files = map[string][]byte{programs(runtime.GOOS, runtime.GOARCH)[0]: fakeProgram("version=1.4.4")}
		}, "is missing", "ack retire restart relaunch", true},
		{"server error", func(_ *testing.T, r *release, _ *recorder) { r.status = map[string]int{"checksums.txt": 500} }, "HTTP 500", "ack restart relaunch", false},
		{"window did not close", func(_ *testing.T, _ *release, rec *recorder) {
			rec.failAt = map[string]error{"wait": errors.New("did not exit")}
		}, "did not exit", "ack wait retire restart relaunch", true},
		{"resident would not stop", func(_ *testing.T, _ *release, rec *recorder) {
			rec.failAt = map[string]error{"stop": errors.New("stuck")}
		}, "stop resident: stuck", "ack wait stop retire restart relaunch", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newInstall(t, "version=1.4.3")
			rel := release{version: "1.4.4"}
			rec := &recorder{}
			tc.setup(t, &rel, rec)
			src, _ := rel.serve(t, newProgramsAre("1.4.4"))
			before := tree(t, f.dir)
			home := t.TempDir()
			res, err := applyFrom(t.Context(), home, f.exe, "1.4.3", "1.4.4", rec.hooks(), fakeRun(t), fixedSources(src))
			if err == nil || !strings.Contains(err.Error(), tc.err) || res.Success {
				t.Fatalf("result=%+v err=%v, want %q", res, err, tc.err)
			}
			if rec.order() != tc.steps {
				t.Errorf("steps %q, want %q", rec.order(), tc.steps)
			}
			wantVersions(t, f, "1.4.3")
			f.assertOthersUntouched(t)
			f.assertNoStray(t)
			if folders := f.updateFolders(t); (len(folders) == 1) != tc.folder || len(folders) > 1 {
				t.Errorf("update folders %v", folders)
			}
			if !tc.folder && !reflect.DeepEqual(tree(t, f.dir), before) {
				t.Errorf("the install folder changed:\n%v\n%v", before, tree(t, f.dir))
			}
			if p, _ := ReadProgress(home); p.Phase != PhaseFailed || !strings.Contains(p.Error, tc.err) {
				t.Errorf("final progress %+v", p)
			}
		})
	}
}

func TestManualUpdateRejectsAnInstallThatChangedUnderneath(t *testing.T) {
	f := newInstall(t, "version=1.5.0") // not the version this run was started from
	src, _ := release{version: "1.4.4"}.serve(t, newProgramsAre("1.4.4"))
	rec := &recorder{}
	_, err := applyFrom(t.Context(), t.TempDir(), f.exe, "1.4.3", "1.4.4", rec.hooks(), fakeRun(t), fixedSources(src))
	if err == nil || !strings.Contains(err.Error(), "installed version changed") || strings.Contains(rec.order(), "stop") {
		t.Fatalf("err=%v steps=%s", err, rec.order())
	}
	wantVersions(t, f, "1.5.0")
	f.assertOthersUntouched(t)
}

// failRename makes the n-th rename (counting from 1) fail, and every one after
// it when from is set. The previous rename function is restored afterwards.
func failRename(t *testing.T, n int, from bool) *int {
	t.Helper()
	calls := 0
	previous := rename
	rename = func(oldpath, newpath string) error {
		calls++
		if calls == n || (from && calls > n) {
			return fmt.Errorf("injected failure at rename %d", calls)
		}
		return previous(oldpath, newpath)
	}
	t.Cleanup(func() { rename = previous })
	return &calls
}

// Whichever step of the swap fails, the previous programs come back.
func TestManualUpdateRollsBackFromEveryStep(t *testing.T) {
	steps := 2 * len(programs(runtime.GOOS, runtime.GOARCH))
	for n := 1; n <= steps; n++ {
		t.Run(fmt.Sprintf("rename %d of %d", n, steps), func(t *testing.T) {
			f := newInstall(t, "version=1.4.3")
			src, _ := release{version: "1.4.4"}.serve(t, newProgramsAre("1.4.4"))
			failRename(t, n, false)
			rec := &recorder{}
			res, err := applyFrom(t.Context(), t.TempDir(), f.exe, "1.4.3", "1.4.4", rec.hooks(), fakeRun(t), fixedSources(src))
			if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("injected failure at rename %d", n)) || !strings.Contains(err.Error(), "put back") || res.Success {
				t.Fatalf("result=%+v err=%v", res, err)
			}
			if rec.order() != "ack wait stop retire restart relaunch" {
				t.Errorf("steps %q: the resident must be brought back", rec.order())
			}
			wantVersions(t, f, "1.4.3")
			f.assertOthersUntouched(t)
			f.assertNoStray(t)
			// The new programs stay staged, the old ones are home again.
			folders := f.updateFolders(t)
			if len(folders) != 1 {
				t.Fatalf("update folders %v", folders)
			}
			if entries, _ := os.ReadDir(filepath.Join(folders[0], "old")); len(entries) != 0 {
				t.Errorf("old/ holds %d files after rollback", len(entries))
			}
			if entries, _ := os.ReadDir(filepath.Join(folders[0], "new")); len(entries) != steps/2 {
				t.Errorf("new/ holds %d files, want all %d staged again", len(entries), steps/2)
			}
		})
	}
}

func TestManualUpdateRollsBackWhenTheNewProgramFails(t *testing.T) {
	for name, files := range map[string]func(string) []byte{
		"does not start": func(n string) []byte {
			if n == programs(runtime.GOOS, runtime.GOARCH)[0] {
				return fakeProgram("crash")
			}
			return fakeProgram("version=1.4.4")
		},
		"reports the old version": func(string) []byte { return fakeProgram("version=1.4.3") },
		"prints nothing":          func(string) []byte { return fakeProgram("silent") },
	} {
		t.Run(name, func(t *testing.T) {
			f := newInstall(t, "version=1.4.3")
			src, _ := release{version: "1.4.4"}.serve(t, files)
			rec := &recorder{}
			res, err := applyFrom(t.Context(), t.TempDir(), f.exe, "1.4.3", "1.4.4", rec.hooks(), fakeRun(t), fixedSources(src))
			if err == nil || res.Success || !strings.Contains(err.Error(), "put back") {
				t.Fatalf("result=%+v err=%v", res, err)
			}
			if rec.order() != "ack wait stop retire restart relaunch" {
				t.Errorf("steps %q", rec.order())
			}
			wantVersions(t, f, "1.4.3")
			f.assertOthersUntouched(t)
			f.assertNoStray(t)
		})
	}
}

// When even the rollback fails the old programs stay where they are, named in
// the error, and the folder is not given a lifespan.
func TestManualUpdateKeepsThePreviousProgramsWhenRollbackFails(t *testing.T) {
	names := programs(runtime.GOOS, runtime.GOARCH)
	f := newInstall(t, "version=1.4.3")
	src, _ := release{version: "1.4.4"}.serve(t, func(string) []byte { return fakeProgram("crash") })
	failRename(t, 2*len(names)+1, true)
	rec := &recorder{}
	_, err := applyFrom(t.Context(), t.TempDir(), f.exe, "1.4.3", "1.4.4", rec.hooks(), fakeRun(t), fixedSources(src))
	if err == nil || !strings.Contains(err.Error(), "could not all be put back") {
		t.Fatal(err)
	}
	if strings.Contains(rec.order(), "retire") {
		t.Errorf("a folder holding the only copy was given a lifespan: %s", rec.order())
	}
	folders := f.updateFolders(t)
	if len(folders) != 1 || !strings.Contains(err.Error(), "old") {
		t.Fatalf("folders %v, err %v", folders, err)
	}
	for _, p := range names {
		data, err := os.ReadFile(filepath.Join(folders[0], "old", p))
		if err != nil || string(data) != string(fakeProgram("version=1.4.3")) {
			t.Errorf("the previous %s is not in old/: %q %v", p, data, err)
		}
	}
	f.assertOthersUntouched(t)
}

func TestManualUpdateFallsBackToTheMirrorOnlyWhenGitHubDoesNotAnswer(t *testing.T) {
	good, _ := release{version: "1.4.4"}.serve(t, newProgramsAre("1.4.4"))
	t.Run("no answer", func(t *testing.T) {
		f := newInstall(t, "version=1.4.3")
		src := sources{github: closedServerURL(t) + "/v1.4.4/", mirror: good.mirror, client: good.client}
		res, err := applyFrom(t.Context(), t.TempDir(), f.exe, "1.4.3", "1.4.4", (&recorder{}).hooks(), fakeRun(t), fixedSources(src))
		if err != nil || !res.Success {
			t.Fatalf("result=%+v err=%v", res, err)
		}
		if data, _ := os.ReadFile(res.LogPath); !strings.Contains(string(data), "GitHub did not answer") {
			t.Errorf("log: %s", data)
		}
		wantVersions(t, f, "1.4.4")
	})
	t.Run("an HTTP error is an answer", func(t *testing.T) {
		f := newInstall(t, "version=1.4.3")
		github, _ := release{version: "1.4.4", status: map[string]int{"checksums.txt": 404}}.serve(t, newProgramsAre("1.4.4"))
		src := sources{github: github.github, mirror: good.mirror, client: good.client}
		_, err := applyFrom(t.Context(), t.TempDir(), f.exe, "1.4.3", "1.4.4", (&recorder{}).hooks(), fakeRun(t), fixedSources(src))
		if err == nil || !strings.Contains(err.Error(), "HTTP 404") {
			t.Fatal(err)
		}
		wantVersions(t, f, "1.4.3")
	})
}

func TestManualUpdateIgnoresEverythingButItsPrograms(t *testing.T) {
	f := newInstall(t, "version=1.4.3")
	files := map[string][]byte{"LICENSE": []byte("new license"), "README.txt": []byte("new"),
		"../evil.exe": []byte("x"), "sub/obit.exe": []byte("nested"), "extra.dll": []byte("dll"), "./notes.txt": []byte("overwrite")}
	for _, p := range programs(runtime.GOOS, runtime.GOARCH) {
		files[p] = fakeProgram("version=1.4.4")
	}
	src, _ := release{version: "1.4.4", files: files}.serve(t, nil)
	res, err := applyFrom(t.Context(), t.TempDir(), f.exe, "1.4.3", "1.4.4", (&recorder{}).hooks(), fakeRun(t), fixedSources(src))
	if err != nil || !res.Success {
		t.Fatalf("result=%+v err=%v", res, err)
	}
	wantVersions(t, f, "1.4.4")
	f.assertOthersUntouched(t) // including LICENSE and notes.txt
	f.assertNoStray(t)
	if _, err := os.Stat(filepath.Join(filepath.Dir(f.dir), "evil.exe")); err == nil {
		t.Error("an entry escaped the install folder")
	}
}

func TestManualUpdateRefusesAnInstallThatCannotUpdate(t *testing.T) {
	f := newInstall(t, "version=1.4.3")
	if err := os.Remove(filepath.Join(f.dir, programs(runtime.GOOS, runtime.GOARCH)[1])); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}
	_, err := applyFrom(t.Context(), t.TempDir(), f.exe, "1.4.3", "1.4.4", rec.hooks(), fakeRun(t), fixedSources(sources{}))
	var blocked *BlockedError
	if !errors.As(err, &blocked) || blocked.Reason != BlockedFiles || rec.order() != "" {
		t.Fatalf("err=%v steps=%q", err, rec.order())
	}
}
