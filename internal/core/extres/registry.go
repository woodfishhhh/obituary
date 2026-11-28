package extres

import (
	"maps"
	"slices"
)

// Registry is an immutable lookup table of external-resource providers keyed by kind.
type Registry struct {
	providers map[string]Provider
}

// NewRegistry builds a Registry from providers. Nil providers and providers
// with an empty kind are ignored; later registrations for the same kind win.
func NewRegistry(providers ...Provider) *Registry {
	m := make(map[string]Provider, len(providers))
	for _, p := range providers {
		if p == nil || p.Kind() == "" {
			continue
		}
		m[p.Kind()] = p
	}
	return &Registry{providers: m}
}

// Get returns the provider registered for kind.
func (r *Registry) Get(kind string) (Provider, bool) {
	if r == nil || kind == "" {
		return nil, false
	}
	p, ok := r.providers[kind]
	return p, ok
}

// Kinds returns the registered provider kinds in deterministic sorted order.
func (r *Registry) Kinds() []string {
	if r == nil || len(r.providers) == 0 {
		return nil
	}
	return slices.Sorted(maps.Keys(r.providers))
}

// RetentionPolicyFor returns the optional RetentionPolicy implemented by kind's provider.
func (r *Registry) RetentionPolicyFor(kind string) (RetentionPolicy, bool) {
	p, ok := r.Get(kind)
	if !ok {
		return nil, false
	}
	rp, ok := p.(RetentionPolicy)
	return rp, ok
}
