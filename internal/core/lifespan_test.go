//go:build windows

package core_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestLifespanUnconfirmedLongPathKeepsAdvice(t *testing.T) {
	core.RequireFsync(t)
	for _, action := range []string{"mark", "extend", "spare"} {
		t.Run(action, func(t *testing.T) {
			f := newFixture(t)
			f.svc.Store.Home = filepath.Join(f.svc.Store.Home, strings.Repeat("长目录", 55))
			path := f.file("payload.txt")
			item := f.add(path, time.Hour)
			fault := &os.PathError{Op: "sync", Path: filepath.Join(f.svc.Store.Home, "store.json.tmp"), Err: os.ErrPermission}
			reset := core.SetReplacementSync(func(*os.File) error { return fault })
			defer reset()
			var err error
			switch action {
			case "mark":
				_, err = f.svc.Add([]string{path}, core.AddOptions{ExpiresAt: f.clock.Add(2 * time.Hour)})
			case "extend":
				_, err = f.svc.Extend([]string{fmt.Sprint(item.ID)}, time.Hour)
			case "spare":
				_, err = f.svc.Cancel([]string{fmt.Sprint(item.ID)})
			}
			if !errors.Is(err, fault) {
				t.Fatalf("unconfirmed error lost its cause: %v", err)
			}
			// The public error contract must survive bounded clients (MCP keeps
			// 512 bytes); the detailed PathError may be arbitrarily long.
			prefix := err.Error()[:min(512, len(err.Error()))]
			if !strings.Contains(prefix, "commit unconfirmed") || !strings.Contains(prefix, "may already have taken effect") || !strings.Contains(prefix, "check obit list before retrying") {
				t.Fatalf("long path displaced unconfirmed commit advice: %s", prefix)
			}
		})
	}
}

func TestLifespanRestoreAdviceDoesNotLockLedger(t *testing.T) {
	assertRestoreAdviceDoesNotLockLedger(t, func(f *fixture, b core.Burial) string {
		rows, err := f.svc.Cancel([]string{b.Path})
		if err != nil {
			return err.Error()
		}
		return rows[0].Error
	})
}

func TestLifespanAuditsAfterCancellation(t *testing.T) {
	for _, spare := range []bool{false, true} {
		t.Run(fmt.Sprint(spare), func(t *testing.T) {
			f := newFixture(t)
			path := f.file("lifespan.log")
			old := f.add(path, time.Hour)
			action := core.ActionExtended
			if spare {
				action = core.ActionCanceled
			}
			assertCommittedAudit(t, f, action, func(ctx context.Context) error {
				svc := f.svc.WithContext(ctx)
				if spare {
					_, err := svc.Cancel([]string{path})
					return err
				}
				_, err := svc.Extend([]string{path}, time.Hour)
				return err
			}, func(st core.State) bool {
				if spare {
					return len(st.Items) == 0
				}
				return len(st.Items) == 1 && st.Items[0].ExpiresAt.After(old.ExpiresAt)
			})
		})
	}
}

func TestBuriedChildMutationGivesResurrectionAdvice(t *testing.T) {
	f := newFixture(t)
	path := f.file("output/child.txt")
	parent := f.add(filepath.Dir(path), time.Hour)
	child := f.add(path, time.Hour)
	f.clock = f.clock.Add(time.Hour)
	if _, err := f.svc.Bury([]string{parent.Path}, false, ""); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{parent.ID, child.ID} {
		refs := []string{fmt.Sprint(id)}
		for _, extend := range []bool{false, true} {
			var rows []core.Result
			var err error
			if extend {
				rows, err = f.svc.Extend(refs, time.Hour)
			} else {
				rows, err = f.svc.Cancel(refs)
			}
			if err != nil || !core.Failed(rows) || !strings.Contains(rows[0].Error, fmt.Sprintf("obit resurrect %d", parent.ID)) {
				t.Fatalf("missing resurrection advice: %+v, %v", rows, err)
			}
		}
	}
}

func TestPendingParentBlocksChildMutations(t *testing.T) {
	for _, byIdentity := range []bool{false, true} {
		for _, operation := range []string{"cancel", "extend", "add"} {
			t.Run(fmt.Sprintf("%s/identity=%v", operation, byIdentity), func(t *testing.T) {
				f := newFixture(t)
				path := f.file("output/child.txt")
				parent := f.add(filepath.Dir(path), time.Hour)
				child := f.add(path, time.Hour)
				f.clock = f.clock.Add(time.Hour)
				stop := errors.New("interrupted move")
				restore := core.SetBurialStep(f.svc, func(step string) error {
					if step == "moved" {
						return stop
					}
					return nil
				})
				t.Cleanup(restore)
				if _, err := f.svc.Bury([]string{parent.Path}, false, ""); !errors.Is(err, stop) {
					t.Fatal(err)
				}
				restore()
				pending, err := f.svc.PendingGraves()
				if err != nil || len(pending) != 1 {
					t.Fatalf("pending: %+v %v", pending, err)
				}
				outside := filepath.Join(f.dir, "escaped.txt")
				if err := os.Rename(filepath.Join(pending[0].Burial.Grave, "child.txt"), outside); err != nil {
					t.Fatal(err)
				}
				if byIdentity {
					if err := f.svc.Store.Update(func(st *core.State) error {
						idx := st.Index(child.ID)
						child.ID = st.NextID
						st.NextID++
						st.Items[idx].ID = child.ID
						return nil
					}); err != nil {
						t.Fatal(err)
					}
				}
				before := f.items()
				var rows []core.Result
				err = nil
				switch operation {
				case "cancel":
					rows, err = f.svc.Cancel([]string{fmt.Sprint(child.ID)})
				case "extend":
					rows, err = f.svc.Extend([]string{fmt.Sprint(child.ID)}, time.Hour)
				case "add":
					f.file("output/child.txt")
					rows, err = f.svc.Add([]string{path}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
				}
				want := fmt.Sprintf("#%d is being buried with #%d; obit reap finishes it, then obit resurrect %d", child.ID, parent.ID, parent.ID)
				if err != nil || len(rows) != 1 || !strings.Contains(rows[0].Error, want) || !reflect.DeepEqual(before, f.items()) {
					t.Fatalf("pending child mutation not refused: %+v %v", rows, err)
				}
			})
		}
	}
}

func TestManifestRejectsExtendAndSpareBeforeMove(t *testing.T) {
	for _, operation := range []string{"sweep", "bury"} {
		for _, mutation := range []string{"extend", "spare"} {
			t.Run(operation+"/"+mutation, func(t *testing.T) {
				f := newFixture(t)
				path := f.file("keep.log")
				item := f.add(path, time.Hour)
				f.clock = f.clock.Add(time.Hour)
				calls := 0
				t.Cleanup(core.SetAfterManifest(f.svc, func() {
					calls++
					pending := f.items()
					if len(pending) != 1 || pending[0].ID != item.ID || !reflect.DeepEqual(pending[0], item) {
						t.Fatalf("manifest must not modify the tracked item: %+v", pending)
					}
					var rows []core.Result
					var err error
					ref := []string{fmt.Sprint(item.ID)}
					if mutation == "extend" {
						rows, err = f.svc.Extend(ref, 30*24*time.Hour)
					} else {
						rows, err = f.svc.Cancel(ref)
					}
					if err != nil || !core.Failed(rows) || len(rows) != 1 || !strings.Contains(rows[0].Error, "is being buried") {
						t.Fatalf("concurrent mutation: %+v, %v", rows, err)
					}
					if !reflect.DeepEqual(f.items()[0], item) {
						t.Fatal("rejected mutation changed the item")
					}
				}))
				var action string
				if operation == "sweep" {
					out, err := f.svc.Sweep(false)
					if err != nil || len(out) != 1 {
						t.Fatalf("sweep: %+v, %v", out, err)
					}
					action = out[0].Action
				} else {
					out, err := f.svc.Bury([]string{path}, false, "")
					if (err != nil && out[0].Action != core.ActionSkipped) || len(out) != 1 {
						t.Fatalf("bury: %+v, %v", out, err)
					}
					action = out[0].Action
				}
				if calls != 1 {
					t.Fatalf("manifest seam called %d times; want 1", calls)
				}
				graves := f.burials()
				if action != core.ActionBuried || len(graves) != 1 || len(f.items()) != 0 {
					t.Fatalf("rejected mutation must allow the pending burial to finish: %s, %+v", action, graves)
				}
				gotID, err := f.fs.Identify(graves[0].Grave)
				if err != nil || gotID != *item.Identity || read(t, graves[0].Grave) != "keep.log" {
					t.Fatal("pending burial lost the original object")
				}
			})
		}
	}
}

func TestPendingBurialSurvivesAddAndSpare(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, operation := range []string{"sweep", "bury"} {
		for _, mutation := range []string{"add", "spare", "extend"} {
			t.Run(operation+"/"+mutation, func(t *testing.T) {
				f := newFixture(t)
				path := f.file("original.txt")
				f.add(path, time.Hour)
				f.clock = f.clock.Add(time.Hour)
				item, _ := interruptBurial(t, f, path, operation)
				switch mutation {
				case "add":
					if err := os.WriteFile(path, []byte("new file"), 0o600); err != nil {
						t.Fatal(err)
					}
					rows, err := f.svc.Add([]string{path}, core.AddOptions{ExpiresAt: f.clock.Add(24 * time.Hour)})
					if err != nil || !core.Failed(rows) {
						t.Fatalf("pending burial accepted add: %+v, %v", rows, err)
					}
				case "spare":
					_, _ = f.svc.Cancel([]string{fmt.Sprint(item.ID)})
				default:
					_, _ = f.svc.Extend([]string{fmt.Sprint(item.ID)}, time.Hour)
				}
				if _, _, err := f.svc.Tick(); err != nil {
					t.Fatal(err)
				}
				graves := f.burials()
				if len(graves) != 1 || graves[0].Path != path || graves[0].ID != item.ID {
					t.Fatalf("ordinary mutation lost recovery information: %+v, %+v", graves, f.items())
				}
				sum, err := f.svc.Summarize()
				if err != nil || graves[0].Size != int64(len("original.txt")) || sum.GraveBytes != graves[0].Size || sum.Lifetime.Buried != 1 || sum.Lifetime.BuriedBytes != graves[0].Size {
					t.Fatalf("recovered burial size was not counted: %+v, %+v, %v", graves, sum, err)
				}
				got, err := f.svc.Resurrect([]string{fmt.Sprint(item.ID)})
				if err != nil || core.GraveFailed(got) || !samePath(filepath.Dir(got[0].Path), filepath.Dir(path)) || read(t, got[0].Path) != "original.txt" {
					t.Fatalf("restoration must leave the graveyard: %+v, %v", got, err)
				}
				if mutation == "add" && read(t, path) != "new file" {
					t.Fatal("replacement was overwritten")
				}
			})
		}
	}
}

func TestPendingBurialRejectsLifespanMutations(t *testing.T) {
	for _, mutation := range []string{"extend", "spare", "add-missing", "add-replacement"} {
		t.Run(mutation, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("keep.txt")
			original := f.add(path, time.Hour)
			_, grave := interruptBurial(t, f, path, "bury")
			if mutation == "add-replacement" {
				f.file("keep.txt")
			}
			before, _ := f.svc.Store.Read()
			var rows []core.Result
			var err error
			refs := []string{fmt.Sprint(original.ID)}
			switch mutation {
			case "extend":
				rows, err = f.svc.Extend(refs, 30*24*time.Hour)
			case "spare":
				rows, err = f.svc.Cancel(refs)
			default:
				rows, err = f.svc.Add([]string{path}, core.AddOptions{ExpiresAt: f.clock.Add(30 * 24 * time.Hour)})
			}
			if err != nil || len(rows) != 1 || !core.Failed(rows) || !strings.Contains(rows[0].Error, "is being buried") || !strings.Contains(rows[0].Error, "obit resurrect") {
				t.Fatalf("pending burial accepted mutation or hid recovery advice: %+v, %v", rows, err)
			}
			after, _ := f.svc.Store.Read()
			if !reflect.DeepEqual(before, after) || read(t, grave) != "keep.txt" {
				t.Fatal("rejected mutation changed the ledger or buried object")
			}
			if _, err := f.svc.Sweep(false); err != nil {
				t.Fatal(err)
			}
			restored, err := f.svc.Resurrect(refs)
			if err != nil || core.GraveFailed(restored) || read(t, restored[0].Path) != "keep.txt" || len(f.items()) != 0 {
				t.Fatalf("reap then resurrect failed: %+v, %v", restored, err)
			}
			if mutation != "add-replacement" && !samePath(restored[0].Path, path) {
				t.Fatal("did not return to original path")
			}
		})
	}
}

func TestMutationsRejectChildAlreadyInGraveyard(t *testing.T) {
	for _, mutation := range []string{"extend", "cancel"} {
		t.Run(mutation, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("output/keep.txt")
			parent := f.add(filepath.Dir(path), time.Hour)
			child := f.add(path, time.Hour)
			f.clock = f.clock.Add(time.Hour)
			if _, err := f.svc.Bury([]string{parent.Path}, false, ""); err != nil {
				t.Fatal(err)
			}
			// A ledger left by an older binary can still contain the child.
			if err := f.svc.Store.Update(func(st *core.State) error {
				if st.Index(child.ID) < 0 {
					st.Items = append(st.Items, child)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			before, _ := f.svc.Store.Read()
			refs := []string{fmt.Sprint(child.ID)}
			var rows []core.Result
			var err error
			if mutation == "extend" {
				rows, err = f.svc.Extend(refs, 30*24*time.Hour)
			} else {
				rows, err = f.svc.Cancel(refs)
			}
			if err != nil || !core.Failed(rows) || !strings.Contains(rows[0].Error, fmt.Sprintf("obit resurrect %d", parent.ID)) {
				t.Fatalf("buried child mutation accepted: %+v, %v", rows, err)
			}
			after, _ := f.svc.Store.Read()
			if !reflect.DeepEqual(before, after) {
				t.Fatal("rejected mutation changed ledger")
			}
		})
	}
}

func TestExtendAndCancel(t *testing.T) {
	f := newFixture(t)
	a, b := f.file("a.log"), f.file("b.log")
	itemA := f.add(a, time.Hour)
	f.add(b, time.Hour)

	f.clock = f.clock.Add(2 * time.Hour) // a is overdue: extend counts from now
	if _, err := f.svc.Extend([]string{"#1"}, 30*time.Minute); err != nil {
		t.Fatal(err)
	}
	results, err := f.svc.Cancel([]string{b, "#99"})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Error != "" || results[1].Error == "" {
		t.Fatalf("want cancel of b ok and #99 unknown, got %+v", results)
	}
	items := f.items()
	if len(items) != 1 || items[0].ID != itemA.ID || !items[0].ExpiresAt.Equal(f.clock.Add(30*time.Minute)) {
		t.Fatalf("unexpected items %+v", items)
	}
	if !exists(b) {
		t.Fatal("cancel must keep the file")
	}
}
