package core_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestObservationLegacyImportPreservesRequestsAndTimeOrder(t *testing.T) {
	f := newFixture(t)
	rows, err := f.svc.CaptureObservations([]string{f.file("first.log"), f.file("second.log")}, "s")
	if err != nil {
		t.Fatal(err)
	}
	rows[0].ObservedAt = rows[1].ObservedAt.Add(time.Hour)
	path := filepath.Join(f.svc.Store.Home, "observations.jsonl")
	if err := os.MkdirAll(f.svc.Store.Home, 0o700); err != nil {
		t.Fatal(err)
	}
	var data []byte
	for _, row := range rows {
		line, _ := json.Marshal(row)
		data = append(data, append(line, '\n')...)
	}
	data = append(data, []byte(`{"interrupted":`)...)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := f.svc.Store.InspectObservations()
	if err != nil || before.Pending != 3 || before.Invalid != 1 {
		t.Fatal(before, err)
	}
	if err := f.svc.Store.WithObservationReplay(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	pending, err := f.svc.Store.DeferredObservations()
	if err != nil || len(pending) != 2 || pending[0].Token != rows[1].Token || pending[1].Token != rows[0].Token {
		t.Fatal(pending, err)
	}
	status, err := f.svc.Store.InspectObservations()
	if err != nil || status.Dead != 1 {
		t.Fatal(status, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("legacy handoff not retired", err)
	}
	// Reimport after a crash at the old-file removal boundary remains idempotent.
	if err := os.WriteFile(path, data[:len(data)-len(`{"interrupted":`)], 0o600); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Store.WithObservationReplay(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	pending, err = f.svc.Store.DeferredObservations()
	if err != nil || len(pending) != 2 {
		t.Fatal(pending, err)
	}
}
