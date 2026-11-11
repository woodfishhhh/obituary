package testenv

import (
	"fmt"
	"io"
	"maps"
	"slices"
)

// Empty keys and value types are part of a footprint; an empty key left by a
// failed install is still a leak. No snapshot helper repairs user state.
type desktopFootprint map[string]string

func desktopChanges(before, after desktopFootprint) []string {
	changed := make(map[string]bool)
	for path, value := range before {
		if current, ok := after[path]; !ok || current != value {
			changed[path] = true
		}
	}
	for path, value := range after {
		if original, ok := before[path]; !ok || original != value {
			changed[path] = true
		}
	}
	return slices.Sorted(maps.Keys(changed))
}

func checkDesktop(before desktopFootprint, snapshot func() (desktopFootprint, error), out io.Writer) bool {
	after, err := snapshot()
	if err != nil {
		fmt.Fprintln(out, "test desktop snapshot:", err)
		return false
	}
	changes := desktopChanges(before, after)
	for _, path := range changes {
		fmt.Fprintln(out, "test changed real desktop integration:", path)
	}
	return len(changes) == 0
}
