//go:build windows

package lifecycle_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func TestMarkRestoreAdviceDoesNotLockLedger(t *testing.T) {
	assertRestoreAdviceDoesNotLockLedger(t, func(f *fixture, b core.Burial) string {
		rows, err := f.svc.Add([]string{b.Path}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
		return fmt.Sprint(rows, err)
	})
}

func TestSweepAbsorbsChildrenAtTheirCurrentLocations(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	f := newFixture(t)
	inside := f.file("output/moved-out.txt")
	outside := f.file("moved-in.txt")
	parent := f.add(filepath.Dir(inside), time.Hour)
	leaving, arriving := f.add(inside, time.Minute), f.add(outside, time.Minute)
	kept := filepath.Join(f.dir, "kept.txt")
	if err := os.Rename(inside, kept); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(outside, filepath.Join(parent.Path, "arrived.txt")); err != nil {
		t.Fatal(err)
	}
	// Earlier children are waiting for a retry when their parent is claimed.
	if err := f.svc.Store.Update(func(st *core.State) error {
		for i := range st.Items {
			if st.Items[i].ID != parent.ID {
				st.Items[i].NextAttempt = f.clock.Add(2 * time.Hour)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	out := f.sweepAt(time.Hour)
	if len(out) != 1 || out[0].Action != core.ActionBuried || len(f.items()) != 1 || f.items()[0].ID != leaving.ID || read(t, kept) != "output/moved-out.txt" {
		t.Fatalf("wrong child lifespan retired: out=%+v items=%+v", out, f.items())
	}
	if got := containsIDs(t, f.burials()[0]); !reflect.DeepEqual(got, []int64{arriving.ID}) {
		t.Fatalf("contains=%v", got)
	}
}

func TestManifestChildLookupUncertaintyRetries(t *testing.T) {
	for _, cause := range []error{errors.New("offline"), core.ErrUnsafe, core.ErrNotFound} {
		t.Run(cause.Error(), func(t *testing.T) {
			f := newFixture(t)
			child := f.file("output/child.txt")
			dir := filepath.Dir(child)
			f.add(dir, time.Hour)
			f.add(child, 30*24*time.Hour)
			f.fs.locateErr = cause
			out := f.sweepAt(time.Hour)
			if errors.Is(cause, core.ErrNotFound) {
				if len(out) != 1 || out[0].Action != core.ActionBuried {
					t.Fatalf("definitely absent child prevented burial: %+v", out)
				}
			} else if len(out) != 1 || out[0].Action != core.ActionRetry || !exists(child) || len(f.burials()) != 0 {
				t.Fatalf("uncertain child lookup authorized a move: %+v", out)
			}
		})
	}
}
