package daemonipc

import (
	"context"
	"errors"
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
	}, func() bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	rows := []core.Observation{{Token: "token", Session: "session", Path: "/tmp/file", ObservedAt: time.Now(), Identity: &core.Identity{FileID: "id"}}}
	receipt, err := Submit(context.Background(), home, rows)
	if err != nil || !receipt.Quiet {
		t.Fatalf("receipt %+v: %v", receipt, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || got[0].Token != rows[0].Token {
		t.Fatalf("resident received %+v", got)
	}
}

func TestSubmitWithoutResidentIsUnavailable(t *testing.T) {
	rows := []core.Observation{{Token: "token"}}
	if _, err := Submit(context.Background(), t.TempDir(), rows); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("got %v, want ErrUnavailable so the hook registers directly", err)
	}
}

func TestSubmitDistinguishesRefusalAndSilenceFromAbsence(t *testing.T) {
	home := t.TempDir()
	block := make(chan struct{})
	stop, err := Serve(home, func(rows []core.Observation) error {
		if rows[0].Token == "refuse" {
			return errors.New("queue is full")
		}
		<-block // an acknowledgement that never comes
		return nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	defer close(block) // runs first: stop waits for handlers still blocked on it
	_, err = Submit(context.Background(), home, []core.Observation{{Token: "refuse"}})
	if err == nil || errors.Is(err, ErrUnavailable) {
		t.Fatalf("a refusal is an answer from a resident that is there: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = Submit(ctx, home, []core.Observation{{Token: "silent"}})
	if err == nil || errors.Is(err, ErrUnavailable) {
		t.Fatalf("a silent resident must be queued around, not treated as absent: %v", err)
	}
}
