package doctor

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func TestDiagnoseIdentitiesReportsUnrelatedUnsafeObject(t *testing.T) {
	root := t.TempDir()
	svc := &lifecycle.Service{Store: ledger.Store{Home: filepath.Join(root, "data")}, FS: winfs.FS{}}
	path := filepath.Join(root, "keep.txt")
	clitest.WriteFixture(t, path, "keep")
	rows, err := svc.Add([]string{path}, lifecycle.AddOptions{ExpiresAt: time.Now().Add(24 * time.Hour)})
	if err != nil || lifecycle.Failed(rows) {
		t.Fatalf("add: %+v %v", rows, err)
	}
	want, detail := "PASS", "hard links"
	if runtime.GOOS != "windows" {
		want, detail = "WARN", "not supported"
	}
	if got := english.diagnoseIdentities(svc.Store.Home); got.Status != want {
		t.Fatalf("healthy identity: %+v", got)
	}
	if err := os.Link(path, path+".alias"); err != nil {
		t.Fatal(err)
	}
	if got := english.diagnoseIdentities(svc.Store.Home); got.Status != "WARN" || got.Errors != 1 || !strings.Contains(got.Detail, detail) {
		t.Fatalf("doctor hid an unrelated unsafe identity: %+v", got)
	}
	history, err := svc.Store.History(10)
	if err != nil || len(history) != 1 {
		t.Fatalf("doctor wrote audit: %+v %v", history, err)
	}
}
