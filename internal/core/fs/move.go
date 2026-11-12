package fs

import "fmt"

// MoveMismatchError reports a move whose destination failed verification.
// Retained is nonempty only when the no-replace rollback could not finish;
// callers must retain recovery evidence and never authorize purging it.
// Cause is not unwrapped: a post-move ENOENT cannot prove the tracked inode
// vanished, and a rollback collision must not retry under a new restore name.
type MoveMismatchError struct {
	Source, Retained string
	Cause, Rollback  error
}

func (e *MoveMismatchError) Error() string {
	if e.Retained == "" {
		return fmt.Sprintf("identity mismatch after move; returned to %s without replacement; entry retained for retry: %v", e.Source, e.Cause)
	}
	return fmt.Sprintf("identity mismatch; never purge; content retained at %s; rollback to %s failed: %v; inspect both locations and manually move the retained object to an unused name; see obit doctor: %v", e.Retained, e.Source, e.Rollback, e.Cause)
}
