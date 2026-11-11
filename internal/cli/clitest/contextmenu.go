package clitest

import (
	"fmt"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"reflect"
	"sync"
)

type menuRegistry struct {
	mu    sync.Mutex
	trees map[string]contextmenu.Tree
}

func NewMenuRegistry() contextmenu.Registry {
	return &menuRegistry{trees: map[string]contextmenu.Tree{}}
}
func (r *menuRegistry) Lock() (func(), error)                          { r.mu.Lock(); return r.mu.Unlock, nil }
func (r *menuRegistry) ReadTree(path string) (contextmenu.Tree, error) { return r.trees[path], nil }
func (r *menuRegistry) CompareAndSwap(c contextmenu.Change) error {
	if !reflect.DeepEqual(r.trees[c.Path], c.Before) {
		return fmt.Errorf("context menu changed after planning")
	}
	r.trees[c.Path] = c.After
	return nil
}
