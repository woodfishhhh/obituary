package rules

import (
	"errors"
	"fmt"
	"path/filepath"
)

// ParseError keeps human-readable detail separate from the agent diagnostic.
type ParseError struct {
	Source string
	Line   int
	Kind   string
	Cause  error
}

func (e *ParseError) Error() string { return fmt.Sprintf("%s:%d: %v", e.Source, e.Line, e.Cause) }
func (e *ParseError) Unwrap() error { return e.Cause }

// PauseDiagnostic never interpolates a rules payload or an underlying error.
func PauseDiagnostic(project string, state FileState, err error) string {
	file, line, kind := state.Path, 0, "read"
	if file == "" {
		file = filepath.Join(project, FileName)
	}
	var parsed *ParseError
	if errors.As(err, &parsed) {
		line, kind = parsed.Line, parsed.Kind
	} else if state.Status == Changed || state.Status == Missing {
		kind = state.Status
	}
	return fmt.Sprintf("Obituary: project %q; file %q; line %d; %s; automatic registration paused.", project, file, line, kind)
}
