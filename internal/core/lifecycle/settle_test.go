package lifecycle_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func TestWatchExpiryAlwaysUsesRestorableGraveyard(t *testing.T) {
	f := newFixture(t)
	f.svc.Owner = auto.Owner
	path := f.file("downloads/file")
	p, err := f.svc.SetWatch(filepath.Dir(path), "1h", core.WatchFirstSeen)
	if err != nil {
		t.Fatal(err)
	}
	approveWatch(t, f.svc)
	if err := f.svc.RegisterWatchChild(p, path); err != nil {
		t.Fatal(err)
	}
	f.svc.Mode = core.ModeRecycle
	out := f.sweepAt(2 * time.Hour)
	if runtime.GOOS != "windows" {
		requireRetainedRetry(t, f, out)
		if read(t, path) != "downloads/file" || len(f.fs.recycled) != 0 || !strings.Contains(out[0].Detail, "excluding concurrent writers") {
			t.Fatalf("unsupported watch exclusion moved the source: %+v", out)
		}
		return
	}
	if len(out) != 1 || out[0].Action != core.ActionBuried || out[0].Burial == nil || !exists(out[0].Grave) {
		t.Fatalf("folder policy must use restorable burial: %+v", out)
	}
	res, err := f.svc.Resurrect([]string{fmt.Sprint(out[0].Item.ID)})
	if err != nil || len(res) != 1 || res[0].Error != "" || !exists(path) {
		t.Fatalf("folder policy child cannot be restored: %+v %v", res, err)
	}
}

func TestPendingBurialRetryUsesDiscoverableRoot(t *testing.T) {
	if runtime.GOOS != "windows" {
		svc := &core.Service{Store: core.Store{Home: t.TempDir()}}
		if got := core.PendingBurialRetry(svc, core.State{}, filepath.Join(t.TempDir(), "payload.txt")); got != "obit reap" {
			t.Fatalf("POSIX root is discoverable through the home: %s", got)
		}
		return
	}
	svc := &core.Service{Store: core.Store{Home: `C:\isolated-home`}}
	path := `D:\work\payload.txt`
	for _, tc := range []struct {
		name  string
		state core.State
		reap  bool
	}{
		{"first volume", core.State{}, false},
		{"tracked volume", core.State{Items: []core.Item{{Path: `D:\work\tracked.txt`}}}, true},
		{"buried volume", core.State{Burials: []core.Burial{{Path: `D:\work\buried.txt`}}}, true},
		{"home volume", core.State{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := path
			if tc.name == "home volume" {
				input = `C:\work\payload.txt`
			}
			got := core.PendingBurialRetry(svc, tc.state, input)
			if strings.Contains(got, "obit reap") != tc.reap || !tc.reap && !strings.Contains(got, "obit bury") {
				t.Fatalf("undiscoverable burial received wrong retry advice: %s", got)
			}
		})
	}
}

func TestRemovedNamedStreamNeverBuriesHost(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("NTFS alternate streams share their host's identity")
	}
	for _, extended := range []bool{false, true} {
		t.Run(fmt.Sprint(extended), func(t *testing.T) {
			f := newFixture(t)
			path := f.file("document.txt")
			stream := path + ":review"
			if extended {
				stream = `\\?\` + stream
			}
			if err := os.WriteFile(stream, []byte("review"), 0600); err != nil {
				t.Fatal(err)
			}
			identity, err := f.fs.Identify(stream)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.svc.Store.Update(func(st *core.State) error {
				st.Items = []core.Item{{ID: 1, Rev: 1, Path: stream, Identity: &identity, ExpiresAt: f.clock.Add(-time.Hour)}}
				st.NextID = 2
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(stream); err != nil {
				t.Fatal(err)
			}
			out, err := f.svc.Sweep(false)
			if err != nil || len(out) != 1 || out[0].Action != core.ActionRetry || !strings.Contains(out[0].Detail, "whole files or directories") || !exists(path) || len(f.burials()) != 0 || len(f.items()) != 1 {
				t.Fatalf("removed named stream selected host file: %+v %v", out, err)
			}
		})
	}
}

func TestPOSIXColonNameDoesNotSelectNeighbor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a colon names an ordinary POSIX file, not an NTFS stream")
	}
	f := newFixture(t)
	host, path := f.file("document.txt"), f.file("document.txt:review")
	rows, err := f.svc.Bury([]string{path}, false, "")
	if err != nil || len(rows) != 1 || rows[0].Action != core.ActionBuried || read(t, host) != "document.txt" || read(t, rows[0].Burial.Grave) != "document.txt:review" {
		t.Fatalf("colon spelling selected its neighbor: %+v %v", rows, err)
	}
	rows, err = f.svc.Resurrect([]string{path})
	if err != nil || core.GraveFailed(rows) || read(t, path) != "document.txt:review" || read(t, host) != "document.txt" {
		t.Fatalf("colon spelling was not restored literally: %+v %v", rows, err)
	}
}

func TestRecycleRechecksClaimBeforeMoving(t *testing.T) {
	for _, operation := range []string{"sweep", "bury"} {
		for _, mutation := range []string{"extend", "spare"} {
			t.Run(operation+"/"+mutation, func(t *testing.T) {
				f := newFixture(t)
				f.svc.Mode = core.ModeRecycle
				path := f.file("keep.txt")
				item := f.add(path, time.Hour)
				f.clock = f.clock.Add(time.Hour)
				calls, trigger := 0, 1
				if operation == "bury" {
					trigger = 2 // inspection followed by the selected item
				}
				var changed core.Item
				f.svc.FS = &identifyHookFS{testFS: f.fs, after: func(found string) {
					if !samePath(found, path) {
						return
					}
					calls++
					if calls != trigger {
						return
					}
					var got []core.Result
					var err error
					if mutation == "extend" {
						got, err = f.svc.Extend([]string{fmt.Sprint(item.ID)}, 30*24*time.Hour)
					} else {
						got, err = f.svc.Cancel([]string{fmt.Sprint(item.ID)})
					}
					if err != nil || core.Failed(got) {
						t.Fatalf("mutation: %+v, %v", got, err)
					}
					changed = *got[0].Item
				}}
				if operation == "sweep" {
					_, _ = f.svc.Sweep(false)
				} else {
					_, _ = f.svc.Bury([]string{path}, false, "")
				}
				items := f.items()
				if changed.ID == 0 || !exists(path) || len(f.fs.recycled) != 0 {
					t.Fatalf("pre-move mutation did not prevent recycling: %+v", items)
				}
				if mutation == "spare" && len(items) != 0 || mutation == "extend" && (len(items) != 1 || !reflect.DeepEqual(items[0], changed)) {
					t.Fatalf("new record lost: %+v", items)
				}
			})
		}
	}
}
