package core_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestWatchDownloadMarkers(t *testing.T) {
	f := newFixture(t)
	for _, suffix := range []string{".td", ".td.cfg", ".xltd", ".!qB", ".!ut", ".!bt", ".bc!", ".opdownload", ".aria2", ".crswap", ".crdownload", ".part", ".partial", ".download", ".filepart", ".tmp"} {
		for _, name := range []string{"file" + suffix, "file" + strings.ToUpper(suffix)} {
			if err := core.CheckWatchDownload(filepath.Join(f.dir, name)); err == nil {
				t.Fatalf("temporary download marker accepted: %s", name)
			}
		}
	}
	for _, name := range []string{"DwnlData", "dwnldata/user/parts/file.zip"} {
		if err := core.CheckWatchDownload(filepath.Join(f.dir, name)); err == nil {
			t.Fatalf("IDM temporary directory accepted: %s", name)
		}
	}
	path := f.file("completed.zip")
	if err := core.CheckWatchDownload(path); err != nil {
		t.Fatal(err)
	}
	f.file("completed.zip.ARIA2")
	if err := core.CheckWatchDownload(path); err == nil {
		t.Fatal("aria2 companion did not protect final name")
	}
}

func TestWatchDownloadNameAllowsExplicitAdd(t *testing.T) {
	f, path, _ := watchFixture(t, "source.td", core.WatchFirstSeen)
	f.add(path, time.Hour)
	out := f.sweepAt(2 * time.Hour)
	if exists(path) || len(out) != 1 || out[0].Action != core.ActionBuried {
		t.Fatalf("download heuristic blocked explicit management: %+v", out)
	}
}
