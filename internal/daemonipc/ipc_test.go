package daemonipc

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

func TestSubmitWaitsForResidentQueueAcceptance(t *testing.T) {
	home := t.TempDir()
	var mu sync.Mutex
	var got []ledger.Observation
	stop, err := Serve(home, func(rows []ledger.Observation) error {
		mu.Lock()
		got = append(got, rows...)
		mu.Unlock()
		return nil
	}, func() bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	rows := []ledger.Observation{{Token: "token", Session: "session", Path: "/tmp/file", ObservedAt: time.Now(), Identity: &corefs.Identity{FileID: "id"}}}
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
	rows := []ledger.Observation{{Token: "token"}}
	if _, err := Submit(context.Background(), t.TempDir(), rows); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("got %v, want ErrUnavailable so the hook registers directly", err)
	}
}

func TestSubmitDistinguishesRefusalAndSilenceFromAbsence(t *testing.T) {
	home := t.TempDir()
	block := make(chan struct{})
	stop, err := Serve(home, func(rows []ledger.Observation) error {
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
	_, err = Submit(context.Background(), home, []ledger.Observation{{Token: "refuse"}})
	if err == nil || errors.Is(err, ErrUnavailable) {
		t.Fatalf("a refusal is an answer from a resident that is there: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = Submit(ctx, home, []ledger.Observation{{Token: "silent"}})
	if err == nil || errors.Is(err, ErrUnavailable) {
		t.Fatalf("a silent resident must be queued around, not treated as absent: %v", err)
	}
}

func TestListIsAnsweredByTheResident(t *testing.T) {
	home := t.TempDir()
	var got ListRequest
	stop, err := Serve(home, nil, nil, WithList(func(req ListRequest) ([]byte, error) {
		got = req
		return []byte("rows\n"), nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	want := ListRequest{Language: "zh-CN", JSON: true, StableWindow: 30 * time.Minute}
	out, err := List(context.Background(), home, want)
	if err != nil || string(out) != "rows\n" || got != want {
		t.Fatalf("out %q err %v request %+v", out, err, got)
	}
}

func TestListFallsBackWhenNobodyCanAnswer(t *testing.T) {
	if _, err := List(context.Background(), t.TempDir(), ListRequest{}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("no resident: %v", err)
	}
	for name, options := range map[string][]Option{
		"a resident without lists": nil, // a resident of an earlier build refuses the request the same way
		"a refusal":                {WithList(func(ListRequest) ([]byte, error) { return nil, errors.New("ledger is busy") })},
		"an empty answer":          {WithList(func(ListRequest) ([]byte, error) { return nil, nil })},
	} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			stop, err := Serve(home, func([]ledger.Observation) error { return nil }, nil, options...)
			if err != nil {
				t.Fatal(err)
			}
			defer stop()
			if out, err := List(context.Background(), home, ListRequest{}); err == nil {
				t.Fatalf("a list the resident did not print must be an error so the command reads the ledger: %q", out)
			}
		})
	}
}

func TestListRequestsAreNotObservations(t *testing.T) {
	home := t.TempDir()
	delivered := false
	stop, err := Serve(home, func([]ledger.Observation) error { delivered = true; return nil }, nil,
		WithList(func(ListRequest) ([]byte, error) { return []byte("x"), nil }))
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	// A request that mixes both is refused whole; the observations are not delivered.
	conn, err := dial(context.Background(), home)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	mixed := request{Version: protocolVersion, Observations: []ledger.Observation{{Token: "t"}}, List: &ListRequest{}}
	if err := json.NewEncoder(conn).Encode(mixed); err != nil {
		t.Fatal(err)
	}
	var reply response
	if err := json.NewDecoder(conn).Decode(&reply); err != nil || reply.Error == "" || delivered {
		t.Fatalf("reply %+v err %v delivered %v", reply, err, delivered)
	}
}
