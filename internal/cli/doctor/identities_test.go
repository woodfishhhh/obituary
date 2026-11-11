package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func TestDiagnoseIdentitiesReportsUnrelatedUnsafeObject(t *testing.T) {
	root := t.TempDir()
	svc := &core.Service{Store: core.Store{Home: filepath.Join(root, "data")}, FS: winfs.FS{}}
	path := filepath.Join(root, "keep.txt")
	clitest.WriteFixture(t, path, "keep")
	rows, err := svc.Add([]string{path}, core.AddOptions{ExpiresAt: time.Now().Add(24 * time.Hour)})
	if err != nil || core.Failed(rows) {
		t.Fatalf("add: %+v %v", rows, err)
	}
	if got := diagnoseIdentities(svc.Store.Home); got.Status != "PASS" {
		t.Fatalf("healthy identity: %+v", got)
	}
	if err := os.Link(path, path+".alias"); err != nil {
		t.Fatal(err)
	}
	if got := diagnoseIdentities(svc.Store.Home); got.Status != "WARN" || got.Errors != 1 || !strings.Contains(got.Detail, "hard links") {
		t.Fatalf("doctor hid an unrelated unsafe identity: %+v", got)
	}
	history, err := svc.Store.History(10)
	if err != nil || len(history) != 1 {
		t.Fatalf("doctor wrote audit: %+v %v", history, err)
	}
}
