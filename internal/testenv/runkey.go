package testenv

import (
	"fmt"
	"sync"

	"github.com/woodfishhhh/obituary/internal/runkey"
)

// RunRegistry is process-local and is installed for every TestMain, including
// packages that only reach startup indirectly through CLI integration helpers.
type RunRegistry struct {
	mu      sync.Mutex
	install sync.Mutex
	values  map[string]string
}

func NewRunRegistry() *RunRegistry { return &RunRegistry{values: make(map[string]string)} }

func (r *RunRegistry) Get(name string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.values[name], nil
}

func (r *RunRegistry) Set(name, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values[name] = value
	return nil
}

func (r *RunRegistry) Lock() (func(), error) {
	r.install.Lock()
	return r.install.Unlock, nil
}

func (r *RunRegistry) Read(name string) (runkey.Value, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.values[name]
	if !ok {
		return runkey.Value{}, nil
	}
	return runkey.StringValue(value), nil
}

func (r *RunRegistry) CompareAndSwap(name string, before, after runkey.Value) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	current := runkey.Value{}
	if value, ok := r.values[name]; ok {
		current = runkey.StringValue(value)
	}
	if current != before {
		return fmt.Errorf("run value changed since planning")
	}
	if after.Exists {
		r.values[name] = after.Text
	} else {
		delete(r.values, name)
	}
	return nil
}

func (r *RunRegistry) Delete(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.values, name)
	return nil
}
