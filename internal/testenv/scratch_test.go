//go:build windows

package testenv_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/testenv"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func TestScratchReapingOwnershipAndAge(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	live := currentScratchOwner(t)
	reused := live
	reused.Created++
	for _, tc := range []struct {
		name    string
		owner   *testenv.ScratchOwner
		age     time.Duration
		removed bool
	}{
		{"exited", &testenv.ScratchOwner{PID: ^uint32(0), Created: 1, Package: "gone"}, 11 * time.Minute, true},
		{"reused-pid", &reused, 11 * time.Minute, true},
		{"live", &live, 48 * time.Hour, false},
		{"recent", &reused, 9 * time.Minute, false},
		{"exact-ten-minutes", &reused, 10 * time.Minute, false},
		{"legacy-old", nil, 25 * time.Hour, true},
		{"legacy-recent", nil, 23 * time.Hour, false},
		{"legacy-exact-day", nil, 24 * time.Hour, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			temp := t.TempDir()
			root := makeScratch(t, temp, "obit-tests-123", tc.owner, now.Add(-tc.age))
			var log bytes.Buffer
			testenv.ReapScratch(temp, now, &log)
			_, err := os.Stat(root)
			if os.IsNotExist(err) != tc.removed {
				t.Fatalf("removed=%v, want %v; stat=%v; log=%s", os.IsNotExist(err), tc.removed, err, &log)
			}
		})
	}
}

func TestScratchMainRecordsOwner(t *testing.T) {
	data, err := userdirs.Data()
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(filepath.Dir(data), testenv.ScratchOwnerFile))
	if err != nil {
		t.Fatal(err)
	}
	var owner testenv.ScratchOwner
	if err := json.Unmarshal(contents, &owner); err != nil {
		t.Fatal(err)
	}
	if owner.PID != uint32(os.Getpid()) || owner.Created == 0 || owner.Package != "github.com/woodfishhhh/obituary/internal/testenv_test" {
		t.Fatalf("wrong TestMain owner: %+v", owner)
	}
	if alive, err := testenv.ScratchOwnerAlive(owner); err != nil || !alive {
		t.Fatalf("owner not alive: %v %v", alive, err)
	}
}

func TestScratchReapingPreservesRecentDescendant(t *testing.T) {
	temp := t.TempDir()
	owner := currentScratchOwner(t)
	owner.Created++
	root := makeScratch(t, temp, "obit-tests-1", &owner, time.Now().Add(-48*time.Hour))
	file := filepath.Join(root, "keep.txt")
	if err := os.WriteFile(file, []byte("new work"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(root, old, old); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	testenv.ReapScratch(temp, time.Now(), &log)
	if data, err := os.ReadFile(file); err != nil || string(data) != "new work" {
		t.Fatalf("recent work changed: %q %v", data, err)
	}
}

func TestScratchReapingOnlyDirectNumericDirectories(t *testing.T) {
	temp := t.TempDir()
	old := time.Now().Add(-48 * time.Hour)
	for _, name := range []string{"obit-tests-", "obit-tests-abc", "obit-tests-123-extra", "obit-tests-１２３", "OBIT-tests-123", "parent/obit-tests-123"} {
		makeScratch(t, temp, name, nil, old)
	}
	out := makeScratch(t, t.TempDir(), "obit-tests-321", nil, old)
	regular := filepath.Join(temp, "obit-tests-456")
	if err := os.WriteFile(regular, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	testenv.ReapScratch(temp, time.Now(), &log)
	for _, path := range []string{out, regular, filepath.Join(temp, "parent/obit-tests-123"), filepath.Join(temp, "obit-tests-abc")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(path, err)
		}
	}
	entries, err := os.ReadDir(temp)
	if err != nil || len(entries) != 7 {
		t.Fatalf("changed non-candidates: %v %v", entries, err)
	}
}

func TestScratchReapingMalformedOwnerIsNotLegacy(t *testing.T) {
	for _, data := range []string{"{", `{}`, `{"pid":1,"created":0,"package":"x"}`, `{"pid":0,"created":1,"package":"x"}`, `{"pid":1,"created":1}`} {
		t.Run(data, func(t *testing.T) {
			temp := t.TempDir()
			root := makeScratch(t, temp, "obit-tests-1", nil, time.Now().Add(-48*time.Hour))
			if err := os.WriteFile(filepath.Join(root, testenv.ScratchOwnerFile), []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			ageScratch(t, root, time.Now().Add(-48*time.Hour))
			var log bytes.Buffer
			testenv.ReapScratch(temp, time.Now(), &log)
			if _, err := os.Stat(root); err != nil {
				t.Fatal(err)
			}
			if log.Len() == 0 {
				t.Fatal("invalid owner was not logged")
			}
		})
	}
}

func currentScratchOwner(t *testing.T) testenv.ScratchOwner {
	t.Helper()
	root := t.TempDir()
	if err := testenv.WriteScratchOwner(root, "testenv_test"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, testenv.ScratchOwnerFile))
	if err != nil {
		t.Fatal(err)
	}
	var owner testenv.ScratchOwner
	if err := json.Unmarshal(data, &owner); err != nil {
		t.Fatal(err)
	}
	return owner
}

func makeScratch(t *testing.T, temp, name string, owner *testenv.ScratchOwner, at time.Time) string {
	t.Helper()
	root := filepath.Join(temp, name)
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if owner != nil {
		data, err := json.Marshal(owner)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, testenv.ScratchOwnerFile), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "keep.txt"), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	ageScratch(t, root, at)
	return root
}

func ageScratch(t *testing.T, root string, at time.Time) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chtimes(path, at, at)
	})
	if err != nil {
		t.Fatal(err)
	}
}
