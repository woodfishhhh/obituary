package daemonipc

import (
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func row(token string) []core.Observation { return []core.Observation{{Token: token}} }

func TestBatchSharesTheNextCommitAmongRequestsThatWaited(t *testing.T) {
	var mu sync.Mutex
	var commits [][]string
	started, release := make(chan struct{}), make(chan struct{})
	handler := Batch(func(rows []core.Observation) error {
		mu.Lock()
		var tokens []string
		for _, r := range rows {
			tokens = append(tokens, r.Token)
		}
		commits = append(commits, tokens)
		first := len(commits) == 1
		mu.Unlock()
		if first {
			close(started)
			<-release
		}
		return nil
	})
	var wg sync.WaitGroup
	submit := func(token string) {
		wg.Go(func() {
			if err := handler(row(token)); err != nil {
				t.Error(err)
			}
		})
	}
	submit("first")
	<-started // a commit is running: everything that arrives now waits for the next one
	for _, token := range []string{"a", "b", "c", "d"} {
		submit(token)
	}
	time.Sleep(100 * time.Millisecond) // let them enqueue behind the running commit
	close(release)
	wg.Wait()
	if len(commits) != 2 || len(commits[1]) != 4 || !slices.Contains(commits[1], "d") {
		t.Fatalf("commits %v: the four waiting requests should share one", commits)
	}
}

func TestBatchIsolatesABadRequestFromItsNeighbours(t *testing.T) {
	bad := errors.New("bad request")
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	handler := Batch(func(rows []core.Observation) error {
		once.Do(func() { close(started); <-release })
		for _, r := range rows {
			if r.Token == "bad" {
				return bad
			}
		}
		return nil
	})
	results := make(map[string]error)
	var mu sync.Mutex
	var wg sync.WaitGroup
	submit := func(token string) {
		wg.Go(func() {
			err := handler(row(token))
			mu.Lock()
			results[token] = err
			mu.Unlock()
		})
	}
	submit("first")
	<-started
	for _, token := range []string{"good-1", "bad", "good-2"} {
		submit(token)
	}
	time.Sleep(100 * time.Millisecond)
	close(release)
	wg.Wait()
	if results["good-1"] != nil || results["good-2"] != nil || !errors.Is(results["bad"], bad) {
		t.Fatalf("results %v", results)
	}
}
