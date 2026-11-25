package daemonipc

import (
	"sync"

	"github.com/woodfishhhh/obituary/internal/core"
)

type admission struct {
	rows []core.Observation
	done chan error
}

// Batch turns concurrent requests into shared commits: while one commit runs,
// the requests that arrive wait for the next, and all of them are committed
// together. A request that arrives at an idle handler is committed at once, with
// no timer. If a shared commit fails, each request is committed on its own so
// one bad request cannot fail its neighbours.
func Batch(commit Handler) Handler {
	var mu sync.Mutex
	var pending []admission
	running := false
	drain := func() {
		for {
			mu.Lock()
			jobs := pending
			pending = nil
			if len(jobs) == 0 {
				running = false
				mu.Unlock()
				return
			}
			mu.Unlock()
			commitJobs(commit, jobs)
		}
	}
	return func(rows []core.Observation) error {
		done := make(chan error, 1)
		mu.Lock()
		pending = append(pending, admission{rows, done})
		if !running {
			running = true
			go drain()
		}
		mu.Unlock()
		return <-done
	}
}

func commitJobs(commit Handler, jobs []admission) {
	var rows []core.Observation
	for _, job := range jobs {
		rows = append(rows, job.rows...)
	}
	err := commit(rows)
	if err == nil || len(jobs) == 1 {
		for _, job := range jobs {
			job.done <- err
		}
		return
	}
	for _, job := range jobs {
		job.done <- commit(job.rows)
	}
}
