package mcp_test

import (
	"fmt"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/cli/graveyard"
	mcpcmd "github.com/woodfishhhh/obituary/internal/cli/mcp"
	"github.com/woodfishhhh/obituary/internal/cli/purge"
	"github.com/woodfishhhh/obituary/internal/cli/resurrect"
	"github.com/woodfishhhh/obituary/internal/core"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type unreadableReceipt struct {
	core.FS
	grave string
}

func (f *unreadableReceipt) MovePinned(path string, id core.Identity, dest string) error {
	f.grave = dest
	return f.FS.MovePinned(path, id, dest)
}
func (f *unreadableReceipt) OpenBurialDirectory(path string, id core.Identity) (fs.ReadDirFile, error) {
	if path == f.grave {
		return nil, os.ErrPermission
	}
	return f.FS.OpenBurialDirectory(path, id)
}
func TestPendingDirectoryRestoreEntryPoints(t *testing.T) {
	for _, entry := range []string{"cli", "mcp"} {
		t.Run(entry, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, _ := clitest.CommandDir(t)
			svc, err := cmdutil.Open()
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(repo, "directory")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(dir, "member")
			clitest.WriteFixture(t, marker, "member")
			childID, err := svc.FS.Identify(marker)
			if err != nil {
				t.Fatal(err)
			}
			keepUntil := time.Now().UTC().Add(7 * 24 * time.Hour)
			items, err := svc.Add([]string{dir}, core.AddOptions{ExpiresAt: time.Now().Add(24 * time.Hour)})
			if err != nil || core.Failed(items) {
				t.Fatal(items, err)
			}
			backend := svc.FS
			svc.FS = &unreadableReceipt{FS: backend}
			rows, err := svc.Bury([]string{dir}, true, "")
			if err != nil || len(rows) != 1 || rows[0].Action != core.ActionRecoveryPending {
				t.Fatal(rows, err)
			}
			svc.FS = backend
			// Restore a pending receipt with an active registration for the member it contains.
			child := core.Item{ID: 2, Rev: 1, Path: marker, Identity: &childID, ExpiresAt: keepUntil, Note: "keep"}
			if err := svc.Store.UpdateHot(func(st *core.State) error {
				if st.NextID != child.ID {
					return fmt.Errorf("unexpected next ID %d", st.NextID)
				}
				st.Items = append(st.Items, child)
				st.NextID++
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			list, _, err := clitest.Execute(t, graveyard.NewCmd(), "--json")
			if err != nil || !strings.Contains(list, "pending") {
				t.Fatal(list, err)
			}
			if _, _, err = clitest.Execute(t, purge.NewCmd(), "1", "--dry-run"); err == nil {
				t.Fatal("pending grave authorized for purge")
			}
			if entry == "cli" {
				if out, _, err := clitest.Execute(t, resurrect.NewCmd(), "1", "--json"); err != nil || !strings.Contains(out, core.ActionResurrected) {
					t.Fatal(out, err)
				}
			} else {
				session := clitest.ConnectMCP(t, mcpcmd.NewServer(svc, "test"))
				var reply struct{ Results []core.GraveResult }
				clitest.CallMCP(t, session, "obit_resurrect", map[string]any{"refs": []string{"1"}}, false, &reply)
				if len(reply.Results) != 1 || reply.Results[0].Action != core.ActionResurrected {
					t.Fatal(reply)
				}
			}
			data, err := os.ReadFile(marker)
			if err != nil || string(data) != "member" {
				t.Fatal(string(data), err)
			}
			st, err := svc.Store.ReadSnapshot()
			if err != nil || st.Index(2) < 0 || !reflect.DeepEqual(st.Items[st.Index(2)], child) {
				t.Fatal("lost child registration", err)
			}
			pending, err := svc.PendingGraves()
			if err != nil || len(pending) != 0 {
				t.Fatal(pending, err)
			}
		})
	}
}
