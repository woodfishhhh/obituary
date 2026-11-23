package daemonipc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestSubmitWaitsForResidentQueueAcceptance(t *testing.T) {
	home := t.TempDir()
	var mu sync.Mutex
	var got []core.Observation
	stop, err := Serve(home, func(rows []core.Observation) error {
		mu.Lock()
		got = append(got, rows...)
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	rows := []core.Observation{{Token: "token", Session: "session", Path: "/tmp/file", ObservedAt: time.Now(), Identity: &core.Identity{FileID: "id"}}}
	if err := Submit(context.Background(), home, rows); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || got[0].Token != rows[0].Token {
		t.Fatalf("resident received %+v", got)
	}
}
