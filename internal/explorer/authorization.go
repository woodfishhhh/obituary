package explorer

import (
	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"github.com/woodfishhhh/obituary/internal/project"
)

// Scope discovery and child identities belong to one pinned directory, never
// to an identity obtained after a path-based authorization has finished.
type authorization struct {
	Identity corefs.Identity
	InScope  bool
	Project  project.Project
	Items    map[string]corefs.Identity
}
