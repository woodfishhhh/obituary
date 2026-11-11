package doctor

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
)

func TestDoctorUpdateIsInfoAndReadOnly(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OBIT_CONFIG_DIR", filepath.Join(root, "config"))
	clitest.WriteFixture(t, filepath.Join(root, "config", "config.toml"), "[update]\ncheck=false\n")
	home := filepath.Join(root, "data")
	clitest.WriteFixture(t, filepath.Join(home, "update.json"), `{"checkedAt":"2026-11-11T00:00:00Z","latest":{"version":"9.0.0"},"lastError":"offline"}`)
	before := clitest.TreeSnapshot(t, root)
	got := diagnoseUpdate(home)
	if got.Status != "INFO" || !strings.Contains(got.Detail, "check=false") || !strings.Contains(got.Detail, "2026-11-11") || !strings.Contains(got.Detail, "newer=true") || !strings.Contains(got.Detail, "lastError=offline") {
		t.Fatalf("%+v", got)
	}
	after := clitest.TreeSnapshot(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("doctor wrote metadata")
	}
	got = diagnoseUpdate(filepath.Join(root, "missing"))
	if got.Status != "INFO" || !strings.Contains(got.Detail, "last checked=never") {
		t.Fatalf("%+v", got)
	}
}
