package auto

import (
	"context"
	"errors"
	"fmt"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/project"
)

// ReplayObservations uses the same current consent, rules, Git, exemption and
// identity gates as Observe. The consumer lock does not block queue producers.
func (e *Engine) ReplayObservations() error {
	return e.Svc.Store.WithObservationReplay(e.replayObservations)
}

func (e *Engine) replayObservations() error {
	rows, failures := e.Svc.Store.DeferredObservations()
	e.sets = nil
	for _, row := range rows {
		if err := e.Svc.Store.Context().Err(); err != nil {
			return errors.Join(failures, err)
		}
		if err := e.replayObservation(row); err != nil {
			failures = errors.Join(failures, err)
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return failures
			}
		}
	}
	return failures
}

func (e *Engine) replayObservation(row core.Observation) error {
	state, err := e.Svc.Store.Read()
	if err != nil {
		return err
	} // lock contention does not consume a request's retry budget
	if state.ObservationCompleted(&row) {
		return e.Svc.Store.AcknowledgeObservation(row)
	}
	skip, retry := e.observationDecision(row)
	if retry != nil {
		_, err := e.Svc.Store.RetryObservation(row, e.now(), retry)
		return errors.Join(fmt.Errorf("observe %s: %w", row.Path, retry), err)
	}
	// Add/Propose persist their own receipt atomically. Skips have no mutation,
	// but need a receipt too: a later consent change must not revive this request.
	state, err = e.Svc.Store.Read()
	if err != nil {
		return err
	}
	if !state.ObservationCompleted(&row) {
		if err := e.Svc.Store.CompleteObservation(row, e.now(), skip); err != nil {
			return err
		}
	}
	return e.Svc.Store.AcknowledgeObservation(row)
}

func (e *Engine) observationDecision(row core.Observation) (string, error) {
	if row.Attempts >= core.ObservationMaxAttempts {
		return "", errors.New("observation exhausted twenty attempts")
	}
	if e.now().Sub(row.ObservedAt) >= core.ObservationMaxAge {
		return "", errors.New("observation is at least seven days old")
	}
	err := e.Svc.CheckObservation(&row)
	if errors.Is(err, core.ErrNotFound) || errors.Is(err, core.ErrUnsafe) {
		return err.Error(), nil
	}
	if err != nil {
		return "", err
	}
	// Preserve delayed-write semantics: an old event must not release a keep
	// decision. Add/vetUnit still enforce exemptions at replay and commit.
	decisions, _, err := e.observe([]string{row.Path}, row.Session, locations{}, &row)
	if len(decisions) == 0 {
		return "", errors.Join(err, errors.New("observation has no completed decision"))
	}
	switch decisions[0].Action {
	case Registered, Proposed:
		return "", err
	case Skipped:
		return decisions[0].Reason, nil
	default:
		return "", errors.Join(err, errors.New("observation has no completed decision"))
	}
}

// Finish replays timed-out writes before computing the session difference;
// files that existed in its snapshot still need their write events handled.
func (e *Engine) FinishFiles(root, session string) ([]Notice, error) {
	if err := e.ReplayObservations(); err != nil {
		_ = e.Svc.Store.Record(core.Event{Time: e.now(), Action: "observation replay warning", Path: root, Detail: err.Error()})
	}
	return e.Leftovers(root, session)
}

func (e *Engine) Finish(root, session string) ([]Notice, error) {
	var notices []Notice
	_, found, err := project.Find(root)
	if err == nil && found {
		notices, err = e.FinishFiles(root, session)
	}
	containers, dockerErr := e.ContainerLeftovers(root, session)
	return append(notices, containers...), errors.Join(err, dockerErr)
}
