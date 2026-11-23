package auto

import (
	"errors"
	"fmt"

	"github.com/woodfishhhh/obituary/internal/core"
)

// replayBatchRows bounds what one resident commit absorbs, so a long queue
// still makes visible progress and one failure never sits behind thousands of rows.
const replayBatchRows = 32

// decisionRetry marks a row whose decision has to be retried later. It abandons
// the batch it was found in; the attempt is counted afterwards, row by row.
type decisionRetry struct{ error }

func (d decisionRetry) Unwrap() error { return d.error }

// replayBatches is the resident's consumer: the same decisions as the one-shot
// replay, but each group of rows is committed with a single durable ledger
// write and the audit outbox is flushed once for the group. A request file is
// removed only after the commit that registered it; a crash in between is safe
// because the commit also stored the request's receipt.
func (e *Engine) replayBatches(rows []core.Observation) error {
	var failures error
	for len(rows) > 0 {
		batch := rows[:min(len(rows), replayBatchRows)]
		rows = rows[len(batch):]
		err := e.commitBatch(batch)
		if err == nil {
			continue
		}
		if !errors.As(err, new(decisionRetry)) {
			return errors.Join(failures, err)
		}
		failures = errors.Join(failures, e.retryBatch(batch))
	}
	return failures
}

func (e *Engine) commitBatch(rows []core.Observation) error {
	err := e.Svc.Store.ReplayBatch(func(tx core.Store) error {
		// A private copy per attempt: the batch may run again after a conflict.
		svc := *e.Svc
		svc.Store, svc.AfterChange = tx, nil
		engine := *e
		engine.Svc = &svc
		for _, row := range rows {
			if err := engine.applyObservation(row); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := e.Svc.Store.FlushAudit(); err != nil {
		return err
	}
	for _, row := range rows {
		err = errors.Join(err, e.Svc.Store.AcknowledgeObservation(row))
	}
	return err
}

// retryBatch commits the rows of an abandoned batch one at a time: a stale
// identity must not hold back its healthy neighbours.
func (e *Engine) retryBatch(rows []core.Observation) error {
	var failures error
	for _, row := range rows {
		err := e.commitBatch([]core.Observation{row})
		if retry := (decisionRetry{}); errors.As(err, &retry) {
			_, retryErr := e.Svc.Store.RetryObservation(row, e.now(), retry.error)
			err = errors.Join(fmt.Errorf("observe %s: %w", row.Path, retry.error), retryErr)
		}
		failures = errors.Join(failures, err)
	}
	return failures
}
