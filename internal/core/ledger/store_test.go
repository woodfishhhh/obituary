//go:build windows

package ledger_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	core "github.com/woodfishhhh/obituary/internal/core/ledger"
	"golang.org/x/sys/windows"
)

func TestStoreUpdateReportsPostRenameSyncFailure(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "flush-probe")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := core.SyncFileForTesting(file); err != nil {
		t.Fatalf("ordinary tests must skip fsync: %v", err)
	}
	core.RequireFsync(t)
	if err := core.SyncFileForTesting(file); err == nil {
		t.Fatal("durability test did not execute the real fsync failure")
	}
	store := core.Store{Home: t.TempDir()}
	stop := core.SetReplacementSync(func(f *os.File) error {
		// Initial ownership metadata has its own durable replacement. Inject
		// this failure only at the ledger commit that this test exercises.
		if filepath.Base(f.Name()) != "store.json.tmp" {
			return f.Sync()
		}
		// The new ledger must already be visible, proving this is the second
		// flush, after rename, rather than the pre-rename data flush.
		st, err := store.Inspect()
		if err != nil || st.NextID != 23 {
			t.Fatalf("flush ran before replacement: %+v %v", st, err)
		}
		return errors.New("injected post-rename sync failure")
	})
	defer stop()
	err = store.Update(func(st *core.State) error { st.NextID = 23; return nil })
	if err == nil || !strings.Contains(err.Error(), "commit unconfirmed") {
		t.Fatalf("post-rename sync failure reported success: %v", err)
	}
}

func TestStoreRenameFailureKeepsOldLedger(t *testing.T) {
	core.RequireFsync(t)
	store := core.Store{Home: t.TempDir()}
	if err := store.Update(func(st *core.State) error { st.NextID = 11; return nil }); err != nil {
		t.Fatal(err)
	}
	p, _ := windows.UTF16PtrFromString(filepath.Join(store.Home, "store.json"))
	h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	flushed := false
	stop := core.SetReplacementSync(func(file *os.File) error { flushed = true; return file.Sync() })
	defer stop()
	err = store.Update(func(st *core.State) error { st.NextID = 23; return nil })
	st, readErr := store.Inspect()
	if err == nil || !strings.Contains(err.Error(), "replace store") || flushed || readErr != nil || st.NextID != 11 {
		t.Fatalf("failed rename changed or flushed another ledger: %+v %v %v", st, err, readErr)
	}
}

func TestDiagnosticReaderSharesDeleteAccess(t *testing.T) {
	store := core.Store{Home: t.TempDir()}
	if err := store.Update(func(st *core.State) error { st.NextID = 3; return nil }); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(filepath.Join(store.Home, "store.json"))
	if err != nil {
		t.Fatal(err)
	}
	// A rename owns DELETE access. Pin that access while Inspect opens its
	// snapshot, independently of scheduler timing or how long a rename takes.
	h, err := windows.CreateFile(name, windows.DELETE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	st, err := store.Inspect()
	if err != nil || st.NextID != 3 {
		t.Fatalf("diagnostic reader does not share delete access: %+v %v", st, err)
	}
}

func TestStoreReplaceUsesOneDirectory(t *testing.T) {
	for _, literal := range []bool{false, true} {
		t.Run(fmt.Sprint(literal), func(t *testing.T) {
			root := t.TempDir()
			home := filepath.Join(root, "data")
			store := core.Store{Home: home}
			if err := store.Update(func(st *core.State) error { st.NextID = 11; return nil }); err != nil {
				t.Fatal(err)
			}
			dotted := `\\?\` + home + "."
			if literal {
				if err := os.Mkdir(dotted, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(dotted+`\store.json`, []byte("literal directory sentinel"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			alias := core.Store{Home: home + "."}
			if err := alias.Update(func(st *core.State) error { st.NextID = 23; return nil }); err != nil {
				t.Fatalf("directory alias replacement failed: %v", err)
			}
			st, err := store.ReadSnapshot()
			if err != nil || st.NextID != 23 {
				t.Fatalf("directory alias wrote another ledger: %+v %v", st, err)
			}
			if literal && read(t, dotted+`\store.json`) != "literal directory sentinel" {
				t.Fatal("directory alias wrote literal trailing-dot ledger")
			}
		})
	}
}

func TestStoreReplaceLongDirectory(t *testing.T) {
	home := t.TempDir()
	for len(home) < 420 {
		home = filepath.Join(home, strings.Repeat("long", 10))
	}
	home = filepath.Join(home, strings.Repeat("x", 467-len(home)-len("store.json")-1))
	store := core.Store{Home: home}
	for _, next := range []int64{11, 23} {
		if err := store.Update(func(st *core.State) error { st.NextID = next; return nil }); err != nil {
			t.Fatalf("long ledger replacement failed (%d): %v", len(home), err)
		}
	}
	st, err := store.ReadSnapshot()
	if err != nil || st.NextID != 23 {
		t.Fatalf("long ledger replacement lost commit: %+v %v", st, err)
	}
	t.Logf("ledger path length=%d", len(filepath.Join(home, "store.json")))
}

func TestStoreCancellationBeforeCommit(t *testing.T) {
	store := core.Store{Home: t.TempDir()}
	if err := store.Update(func(st *core.State) error { st.NextID = 10; return nil }); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(store.Home, "store.json"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	err = store.WithContext(ctx).Update(func(st *core.State) error {
		st.NextID = 20
		cancel() // preflight finished, but no durable commit yet
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled update committed: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(store.Home, "store.json"))
	if err != nil || string(before) != string(after) {
		t.Fatalf("canceled update changed ledger: %v", err)
	}
}
