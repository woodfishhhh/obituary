//go:build windows

package core_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestLedgerParentChainsCollectedAfterResurrectAndPurge(t *testing.T) {
	f := newFixture(t)
	paths := []string{f.file("same/first.txt"), f.file("same/second.txt"), f.file("other/third.txt")}
	rows, err := f.svc.Bury(paths, false, "")
	if err != nil || core.GraveFailed(rows) || len(rows) != 3 {
		t.Fatalf("bury: %+v %v", rows, err)
	}
	assertParentChainCount(t, f.svc.Store, 2)
	out, err := f.svc.Resurrect([]string{fmt.Sprint(rows[0].Burial.ID)})
	if err != nil || core.GraveFailed(out) || !exists(paths[0]) {
		t.Fatalf("restore shared chain: %+v %v", out, err)
	}
	assertParentChainCount(t, f.svc.Store, 2)
	out, err = f.svc.PurgeRefs([]string{fmt.Sprint(rows[1].Burial.ID)}, false)
	if err != nil || core.GraveFailed(out) {
		t.Fatalf("purge last shared reference: %+v %v", out, err)
	}
	assertParentChainCount(t, f.svc.Store, 1)
	out, err = f.svc.Resurrect([]string{fmt.Sprint(rows[2].Burial.ID)})
	if err != nil || core.GraveFailed(out) || !exists(paths[2]) {
		t.Fatalf("restore final chain: %+v %v", out, err)
	}
	assertParentChainCount(t, f.svc.Store, 0)
}

func assertParentChainCount(t *testing.T, store core.Store, want int) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(store.Home, "store.json"))
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		ParentChains [][]core.BurialParent `json:"parentChains"`
	}
	if err := json.Unmarshal(data, &wire); err != nil || len(wire.ParentChains) != want {
		t.Fatalf("parent table count=%d want=%d err=%v", len(wire.ParentChains), want, err)
	}
}
