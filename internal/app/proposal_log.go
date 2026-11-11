package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
)

type tickErrors struct {
	error
	proposals error
	other     error
}

func (e *tickErrors) Unwrap() error { return e.error }

func joinTickErrors(proposals error, other ...error) error {
	err := errors.Join(append(other, proposals)...)
	if err == nil {
		return nil
	}
	return &tickErrors{error: err, proposals: proposals, other: errors.Join(other...)}
}

// ProposalLogErrors logs project transitions and returns other tick errors.
// Disk state also deduplicates separate --once processes. Tick's original error
// remains unchanged for callers and exit status; cache failures retain it all.
func ProposalLogErrors(home string, logger *log.Logger, tickErr error) error {
	current := map[string]string{}
	remaining, complete := proposalLogState(tickErr, current)
	paused, err := Paused(home)
	if err != nil || paused {
		return errors.Join(tickErr, err)
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		return errors.Join(tickErr, err)
	}
	release, err := core.LockFile(filepath.Join(home, "proposal-log.lock"), true)
	if err != nil {
		return errors.Join(tickErr, err)
	}
	defer release()
	if err := logProposalTransitions(home, logger, current, complete); err != nil {
		return errors.Join(tickErr, fmt.Errorf("proposal log state: %w", err))
	}
	return remaining
}

func proposalLogState(err error, projects map[string]string) (error, bool) {
	var tick *tickErrors
	if errors.As(err, &tick) {
		proposalErr := splitProposalErrors(tick.proposals, projects)
		return errors.Join(tick.other, proposalErr), proposalErr == nil
	}
	remaining := splitProposalErrors(err, projects)
	return remaining, remaining == nil
}

func splitProposalErrors(err error, projects map[string]string) error {
	if err == nil {
		return nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		var other []error
		for _, child := range joined.Unwrap() {
			other = append(other, splitProposalErrors(child, projects))
		}
		return errors.Join(other...)
	}
	var p *auto.ProposalProjectError
	if errors.As(err, &p) {
		projects[p.Project] = strings.ReplaceAll(err.Error(), "\n", "; ")
		return nil
	}
	return err
}

func logProposalTransitions(home string, logger *log.Logger, current map[string]string, complete bool) error {
	root, err := os.OpenRoot(home)
	if err != nil {
		return err
	}
	defer root.Close()
	previous := map[string]string{}
	data, err := root.ReadFile("proposal-log.json")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if len(data) != 0 {
		if err := json.Unmarshal(data, &previous); err != nil {
			return err
		}
	}
	if previous == nil {
		previous = map[string]string{}
	}
	next := maps.Clone(previous)
	for _, project := range slices.Sorted(maps.Keys(current)) {
		if previous[project] != current[project] {
			logger.Printf("proposal project %q: %s", project, current[project])
		}
		next[project] = current[project]
	}
	// An incomplete tick provides no evidence that omitted projects recovered.
	if complete {
		for _, project := range slices.Sorted(maps.Keys(previous)) {
			if _, failed := current[project]; failed {
				continue
			}
			logger.Printf("proposal project %q: refresh available again or no pending candidates", project)
			delete(next, project)
		}
	}
	if maps.Equal(previous, next) {
		return nil
	}
	data, err = json.Marshal(next)
	if err != nil {
		return err
	}
	if err := root.WriteFile("proposal-log.json.tmp", data, 0600); err != nil {
		return err
	}
	return root.Rename("proposal-log.json.tmp", "proposal-log.json")
}
