package lifecycle_test

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRegistrationLocationDiagnosticsPreserveLedger(t *testing.T) {
	f := newFixture(t)
	path := f.file("keep.txt")
	it := f.add(path, 30*24*time.Hour)
	if err := os.Link(path, path+".alias"); err != nil {
		t.Fatal(err)
	}
	before := f.items()
	events, err := f.svc.RegistrationLocationDiagnostics()
	if err != nil || len(events) != 1 || events[0].ID != it.ID || events[0].Action != "identity-error" || !strings.Contains(events[0].Detail, "hard links") {
		t.Fatalf("identity diagnostic missing: %+v %v", events, err)
	}
	if after := f.items(); !reflect.DeepEqual(before, after) {
		t.Fatalf("diagnosis changed a lifespan: %+v", after)
	}
	history, err := f.svc.Store.History(10)
	if err != nil || len(history) != 1 {
		t.Fatalf("read-only diagnosis wrote audit: %+v %v", history, err)
	}
}
