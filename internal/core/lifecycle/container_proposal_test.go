package lifecycle_test

import (
	"strings"
	"sync"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/docker"
)

func TestContainerProposalDeduplicatesConcurrentSessions(t *testing.T) {
	f := newFixture(t)
	id := docker.Identity{ID: strings.Repeat("a", 64), Created: time.Now(), Name: "cache"}
	candidate := core.Candidate{Kind: core.KindContainer, Container: &id, Path: id.Display(), Lifespan: "1d"}
	var wg sync.WaitGroup
	for _, session := range []string{"one", "two"} {
		wg.Go(func() {
			_, _, _, err := f.svc.Propose(core.ProposalContainer, "", session, []core.Candidate{candidate, candidate})
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	st, err := f.svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 || len(st.Proposals[0].Candidates) != 1 {
		t.Fatalf("state=%+v err=%v", st, err)
	}
}
