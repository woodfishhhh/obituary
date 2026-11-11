package explorer

import (
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/project"
)

// Scope discovery and child identities belong to one pinned directory, never
// to an identity obtained after a path-based authorization has finished.
type authorization struct {
	Identity core.Identity
	InScope  bool
	Project  project.Project
	Items    map[string]core.Identity
}
