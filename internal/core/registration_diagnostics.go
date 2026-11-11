package core

import (
	"errors"
	"fmt"
)

// RegistrationLocationDiagnostics inspects all recorded identities without
// changing their lifespans. A file registration may prove some identities
// unrelated and omit their lookup; background ticks and doctor still expose
// those objects' errors. Missing objects keep the original non-error meaning.
func (s *Service) RegistrationLocationDiagnostics() ([]Event, error) {
	state, err := s.Store.Inspect()
	if err != nil {
		return nil, err
	}
	var events []Event
	for _, it := range state.Items {
		if it.Kind == KindContainer {
			continue
		}
		if err := s.Store.Context().Err(); err != nil {
			return events, err
		}
		if it.Identity == nil {
			events = append(events, Event{Time: s.now(), Action: "identity-error", ID: it.ID, Path: it.Path,
				Detail: "registration location check: recorded identity is missing; lifespan retained"})
			continue
		}
		if _, err := s.FS.Locate(*it.Identity); err != nil && !errors.Is(err, ErrNotFound) {
			events = append(events, Event{Time: s.now(), Action: "identity-error", ID: it.ID, Path: it.Path,
				Detail: fmt.Sprintf("registration location check: %v; lifespan retained for retry", err)})
		}
	}
	return events, nil
}
