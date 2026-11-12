//go:build linux || darwin

package lifecycle_test

import (
	"os"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestPOSIXKnownPathRegistration(t *testing.T) {
	f := newFixture(t)
	first := f.add(f.file("first.log"), time.Hour)
	rows, err := f.svc.Add([]string{f.file("second.log")}, core.AddOptions{OnlyNew: true, ExpiresAt: f.clock.Add(time.Hour)})
	items := f.items()
	if err != nil || core.Failed(rows) || len(items) != 2 || items[0].ID != first.ID || !items[0].ExpiresAt.Equal(first.ExpiresAt) {
		t.Fatalf("known tracked path blocked independent registration: %+v %v", rows, err)
	}
}

func TestPOSIXRegistrationChecksKnownTrackedPaths(t *testing.T) {
	if testing.Short() {
		t.Skip("automatic registration queries a real Git repository")
	}
	f := newFixture(t)
	testenv.InitRepo(t, f.dir)
	f.svc.Owner, f.svc.OwnerBatch = auto.Owner, auto.OwnerBatch
	first := f.add(f.file("first.log"), time.Hour)
	second := f.file("second.log")
	opts := core.AddOptions{OnlyNew: true, ExpiresAt: f.clock.Add(time.Hour), Source: "rule", Project: f.dir}
	rows, err := f.svc.Add([]string{second}, opts)
	if err != nil || core.Failed(rows) || len(f.items()) != 2 || !f.items()[0].ExpiresAt.Equal(first.ExpiresAt) {
		t.Fatalf("known tracked object blocked unrelated registration: %+v %v", rows, err)
	}
	if err := os.Rename(first.Path, first.Path+".moved"); err != nil {
		t.Fatal(err)
	}
	third := f.file("third.log")
	rows, err = f.svc.Add([]string{third}, opts)
	if err != nil || !core.Failed(rows) || len(f.items()) != 2 || read(t, first.Path+".moved") != "first.log" || read(t, third) != "third.log" {
		t.Fatalf("unknown tracked location must retain both records and content: %+v %v", rows, err)
	}
}
