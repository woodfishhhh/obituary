package doctor

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

func TestDoctorObservationQueueIsReadOnly(t *testing.T) {
	home := t.TempDir()
	store := ledger.Store{Home: home}
	now := time.Now()
	row := core.Observation{Path: filepath.Join(home, "debug.log"), Session: "s", Token: "request", Identity: &core.Identity{}, ObservedAt: now.Add(-2 * time.Hour)}
	if err := store.DeferObservations([]core.Observation{row}); err != nil {
		t.Fatal(err)
	}
	clitest.WriteFixture(t, filepath.Join(home, "observations", "dead", "failed.json"), "{}")
	clitest.WriteFixture(t, filepath.Join(home, "observations", "half.tmp"), "{")
	before := clitest.TreeSnapshot(t, home)
	check := diagnoseHookQueue(home, now)
	if check.Status != "WARN" || check.Queue.Pending != 1 || check.Queue.Dead != 1 || check.Queue.Partial != 1 || !strings.Contains(check.Detail, "oldest 2h0m0s") {
		t.Fatalf("queue diagnosis: %+v", check)
	}
	if !strings.Contains(check.Detail, "1 pending observation;") || !strings.Contains(check.Detail, "1 incomplete temporary file") || strings.Contains(check.Detail, "1 pending observations") {
		t.Fatal(check.Detail)
	}
	if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, home)) {
		t.Fatal("doctor changed queue")
	}
	absent := filepath.Join(t.TempDir(), "absent")
	if check := diagnoseHookQueue(absent, now); check.Status != "PASS" || check.Queue.Pending != 0 {
		t.Fatal(check)
	}
	if _, err := os.Stat(absent); !os.IsNotExist(err) {
		t.Fatal("doctor created queue", err)
	}
}
