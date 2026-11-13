package cancel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	addcmd "github.com/woodfishhhh/obituary/internal/cli/add"
	burycmd "github.com/woodfishhhh/obituary/internal/cli/bury"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	extendcmd "github.com/woodfishhhh/obituary/internal/cli/extend"
	"github.com/woodfishhhh/obituary/internal/core"
)

func TestBuriedChildCommandsFailWithAdviceInTextAndJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, asJSON := range []bool{false, true} {
		for _, extend := range []bool{false, true} {
			t.Run(fmt.Sprintf("json=%v/extend=%v", asJSON, extend), func(t *testing.T) {
				clitest.IsolateRunner(t)
				root, _ := clitest.CommandDir(t)
				path := filepath.Join(root, "output", "child.txt")
				clitest.WriteFixture(t, path, "keep")
				svc, err := cmdutil.Open()
				if err != nil {
					t.Fatal(err)
				}
				now := time.Now()
				svc.Now = func() time.Time { return now }
				added, err := svc.Add([]string{filepath.Dir(path), path}, core.AddOptions{ExpiresAt: now.Add(time.Hour)})
				if err != nil || core.Failed(added) {
					t.Fatalf("add: %+v %v", added, err)
				}
				now = now.Add(time.Hour)
				if rows, err := svc.Bury([]string{filepath.Dir(path)}, false, ""); err != nil || core.GraveFailed(rows) {
					t.Fatalf("bury: %+v %v", rows, err)
				}
				var cmd *cobra.Command
				args := []string{fmt.Sprint(added[1].Item.ID)}
				if extend {
					cmd = extendcmd.NewCmd()
					args = append(args, "1d")
				} else {
					cmd = NewCmd()
				}
				if asJSON {
					args = append(args, "--json")
				}
				out, stderr, err := clitest.Execute(t, cmd, args...)
				want := fmt.Sprintf("#%d was buried with #%d; obit resurrect %d", added[1].Item.ID, added[0].Item.ID, added[0].Item.ID)
				if err == nil || !strings.Contains(out+stderr, want) {
					t.Fatalf("missing nonzero failure/advice: %s %s %v", out, stderr, err)
				}
				if asJSON {
					var rows []core.Result
					if err := json.Unmarshal([]byte(out), &rows); err != nil || !core.Failed(rows) {
						t.Fatalf("JSON error: %s %v", out, err)
					}
				}
			})
		}
	}
}

func TestPathCommandsSelectCurrentObject(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, action := range []string{"spare", "extend", "bury"} {
		t.Run(action, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			path, keep := filepath.Join(repo, "debug.log"), filepath.Join(repo, "keep.txt")
			clitest.WriteFixture(t, path, "old")
			if out, _, err := clitest.Execute(t, addcmd.NewCmd(), path, "1d"); err != nil {
				t.Fatalf("add old: %s, %v", out, err)
			}
			before, err := store.Read()
			if err != nil {
				t.Fatal(err)
			}
			old := before.Items[0]
			if err := os.Rename(path, keep); err != nil {
				t.Fatal(err)
			}
			clitest.WriteFixture(t, path, "new")
			out, stderr, addErr := clitest.Execute(t, addcmd.NewCmd(), path, "3d")
			if runtime.GOOS != "windows" {
				if addErr == nil || !strings.Contains(out+stderr, "not supported") {
					t.Fatalf("unlocated prior object needs an explicit diagnostic: %s %s %v", out, stderr, addErr)
				}
				st, err := store.ReadSnapshot()
				if err != nil || !reflect.DeepEqual(st.Items, before.Items) || len(st.Burials) != 0 {
					t.Fatalf("failed rebinding changed the old lifespan: %+v %v", st, err)
				}
				for file, want := range map[string]string{path: "new", keep: "old"} {
					if data, err := os.ReadFile(file); err != nil || string(data) != want {
						t.Fatalf("failed rebinding changed %s: %q %v", file, data, err)
					}
				}
				return
			}
			if addErr != nil {
				t.Fatalf("add new: %s, %v", out, addErr)
			}
			st, err := store.Read()
			if err != nil || len(st.Items) != 2 {
				t.Fatalf("two objects: %+v, %v", st, err)
			}
			moved := st.Items[st.Index(old.ID)]
			newID := old.ID + 1
			newExpiry := st.Items[st.Index(newID)].ExpiresAt
			if !strings.EqualFold(moved.Path, keep) || moved.Rev != old.Rev+1 || !moved.ExpiresAt.Equal(old.ExpiresAt) {
				t.Errorf("old path must follow its identity: %+v", moved)
			}
			events, err := store.History(20)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, event := range events {
				if event.Action == "moved" && event.ID == old.ID && strings.EqualFold(event.Path, keep) {
					found = true
				}
			}
			if !found {
				t.Error("renamed object has no moved audit event")
			}
			// Also exercise a ledger written by the previous version.
			if err := store.Update(func(st *core.State) error { st.Items[st.Index(old.ID)].Path = path; return nil }); err != nil {
				t.Fatal(err)
			}
			cmd, args := NewCmd(), []string{path}
			switch action {
			case "extend":
				cmd, args = extendcmd.NewCmd(), []string{path, "1d"}
			case "bury":
				cmd = burycmd.NewCmd()
			}
			if out, _, err := clitest.Execute(t, cmd, args...); err != nil {
				t.Fatalf("%s: %s, %v", action, out, err)
			}
			st, err = store.Read()
			idx := st.Index(old.ID)
			if err != nil || idx < 0 || !st.Items[idx].ExpiresAt.Equal(old.ExpiresAt) {
				t.Fatalf("old object's lifespan changed: %+v, %v", st, err)
			}
			if action == "extend" {
				if len(st.Items) != 2 || !st.Items[st.Index(newID)].ExpiresAt.Equal(newExpiry.Add(24*time.Hour)) {
					t.Fatalf("new object's expiry not extended: %+v", st.Items)
				}
			} else if len(st.Items) != 1 {
				t.Fatalf("new object still tracked: %+v", st.Items)
			}
			if data, err := os.ReadFile(keep); err != nil || string(data) != "old" {
				t.Fatalf("old object changed: %q, %v", data, err)
			}
		})
	}
}

func TestPathReferenceDoesNotFallBackFromExistingObject(t *testing.T) {
	for _, kind := range []string{"untracked", "hardlink", "missing"} {
		t.Run(kind, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			path, keep := filepath.Join(repo, "debug.log"), filepath.Join(repo, "keep.txt")
			clitest.WriteFixture(t, path, "keep")
			if _, _, err := clitest.Execute(t, addcmd.NewCmd(), path, "30d"); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(path, keep); err != nil {
				t.Fatal(err)
			}
			if kind != "missing" {
				clitest.WriteFixture(t, path, "new")
			}
			if kind == "hardlink" {
				if err := os.Link(path, path+".link"); err != nil {
					t.Fatal(err)
				}
			}
			out, stderr, err := clitest.Execute(t, NewCmd(), path)
			resolved := kind == "missing" && runtime.GOOS == "windows"
			if (err == nil) != resolved {
				t.Fatalf("path lookup: %v", err)
			}
			if runtime.GOOS != "windows" && kind == "missing" && !strings.Contains(out+stderr, "not supported") {
				t.Fatalf("missing path silently discarded its unknown object: %s %s", out, stderr)
			}
			st, err := store.Read()
			want := 1
			if resolved {
				want = 0
			}
			if err != nil || len(st.Items) != want {
				t.Fatalf("incorrect ledger fallback: %+v, %v", st.Items, err)
			}
			if data, err := os.ReadFile(keep); err != nil || string(data) != "keep" {
				t.Fatalf("path lookup changed the unlocated object: %q %v", data, err)
			}
		})
	}
}

func TestMissingPathReferenceRejectsAmbiguity(t *testing.T) {
	for _, action := range []string{"spare", "extend"} {
		t.Run(action, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			path := filepath.Join(repo, "absent.log")
			if err := store.Update(func(st *core.State) error {
				for range 2 {
					st.Items = append(st.Items, core.Item{ID: st.NextID, Rev: 1, Path: path})
					st.NextID++
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			cmd, args := NewCmd(), []string{path}
			if action == "extend" {
				cmd, args = extendcmd.NewCmd(), []string{path, "1d"}
			}
			out, stderr, err := clitest.Execute(t, cmd, args...)
			if err == nil || !strings.Contains(out+stderr, "#1") || !strings.Contains(out+stderr, "#2") {
				t.Fatalf("must report candidate IDs: %s %s, %v", out, stderr, err)
			}
			st, err := store.Read()
			if err != nil || len(st.Items) != 2 || st.Items[0].Rev != 1 || st.Items[1].Rev != 1 {
				t.Fatalf("ambiguous reference changed ledger: %+v, %v", st, err)
			}
			if out, _, err := clitest.Execute(t, NewCmd(), fmt.Sprint(st.Items[1].ID)); err != nil {
				t.Fatalf("explicit ID must still work: %s, %v", out, err)
			}
		})
	}
}

func TestPathCommandsIgnoreVolumeCase(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, action := range []string{"spare", "extend", "bury"} {
		for _, asJSON := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%v", action, asJSON), func(t *testing.T) {
				clitest.IsolateRunner(t)
				repo, store := clitest.CommandDir(t)
				path := filepath.Join(repo, "debug.log")
				clitest.WriteFixture(t, path, "keep")
				upper := strings.ToUpper(path[:1]) + path[1:]
				lower := strings.ToLower(path[:1]) + path[1:]
				if _, _, err := clitest.Execute(t, addcmd.NewCmd(), upper, "1d"); err != nil {
					t.Fatal(err)
				}
				before, err := store.Read()
				if err != nil {
					t.Fatal(err)
				}
				item := before.Items[0]
				cmd, args := NewCmd(), []string{lower}
				switch action {
				case "extend":
					cmd, args = extendcmd.NewCmd(), []string{lower, "1d"}
				case "bury":
					cmd = burycmd.NewCmd()
				}
				if asJSON {
					args = append(args, "--json")
				}
				out, stderr, err := clitest.Execute(t, cmd, args...)
				if err != nil {
					t.Fatalf("case-insensitive reference: %s %s, %v", out, stderr, err)
				}
				if asJSON {
					var rows []struct {
						Item   *core.Item   `json:"item"`
						Burial *core.Burial `json:"burial"`
					}
					if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) != 1 {
						t.Fatalf("JSON: %q, %v", out, err)
					}
					if action == "bury" {
						if rows[0].Burial == nil || rows[0].Burial.ID != item.ID {
							t.Fatalf("buried a second ID: %s", out)
						}
					} else if rows[0].Item == nil || rows[0].Item.ID != item.ID {
						t.Fatalf("wrong item: %s", out)
					}
				} else if !strings.Contains(out, fmt.Sprintf("#%d", item.ID)) {
					t.Fatalf("wrong ID: %s", out)
				}
				st, err := store.Read()
				if err != nil {
					t.Fatal(err)
				}
				if action == "extend" {
					if len(st.Items) != 1 || st.Items[0].ID != item.ID || !st.Items[0].ExpiresAt.Equal(item.ExpiresAt.Add(24*time.Hour)) {
						t.Fatalf("wrong extension: %+v", st.Items)
					}
				} else if len(st.Items) != 0 {
					t.Fatalf("old record left behind: %+v", st.Items)
				}
			})
		}
	}
}

func TestRenamedPathCommandsReportCurrentPath(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, action := range []string{"spare", "extend"} {
		for _, asJSON := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%v", action, asJSON), func(t *testing.T) {
				clitest.IsolateRunner(t)
				repo, store := clitest.CommandDir(t)
				old, current := filepath.Join(repo, "old.log"), filepath.Join(repo, "current.log")
				clitest.WriteFixture(t, old, "keep")
				if _, _, err := clitest.Execute(t, addcmd.NewCmd(), old, "1d"); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(old, current); err != nil {
					t.Fatal(err)
				}
				cmd, args := NewCmd(), []string{current}
				if action == "extend" {
					cmd, args = extendcmd.NewCmd(), []string{current, "1d"}
				}
				if asJSON {
					args = append(args, "--json")
				}
				out, _, err := clitest.Execute(t, cmd, args...)
				if err != nil {
					t.Fatal(err)
				}
				if asJSON {
					var rows []core.Result
					if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) != 1 || rows[0].Item == nil || rows[0].Item.Path != current {
						t.Fatalf("stale JSON path: %s, %v", out, err)
					}
				} else if !strings.Contains(out, current) || strings.Contains(out, old) {
					t.Fatalf("stale text path: %s", out)
				}
				events, err := store.History(10)
				if err != nil || events[len(events)-1].Path != current {
					t.Fatalf("stale audit path: %+v, %v", events, err)
				}
				st, err := store.Read()
				if err != nil || (action == "extend" && st.Items[0].Path != current) {
					t.Fatalf("stale ledger path: %+v, %v", st.Items, err)
				}
			})
		}
	}
}
