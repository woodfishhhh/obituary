package core_test

import (
	"testing"
)

func TestObservationsRejectChangedExtendedSpelling(t *testing.T) {
	f := newFixture(t)
	path := f.file("neighbor.log")
	rows, err := f.svc.CaptureObservations([]string{`\\?\` + path + "."}, "s1")
	if err == nil || len(rows) != 0 {
		t.Fatalf("observation selected normalized neighbor: %+v %v", rows, err)
	}
	rows, err = f.svc.CaptureObservations([]string{path}, "s1")
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
	rows[0].Path = `\\?\` + path + "."
	if err := f.svc.CheckObservation(&rows[0]); err == nil {
		t.Fatal("changed deferred spelling accepted")
	}
}
