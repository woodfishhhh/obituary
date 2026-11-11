//go:build linux || darwin

package runkey

import "github.com/woodfishhhh/obituary/internal/core"

type Registry interface {
	Get(string) (string, error)
	Read(string) (Value, error)
	CompareAndSwap(string, Value, Value) error
	Lock() (func(), error)
}

// Value keeps the portable startup plan shape; Kind 1 denotes REG_SZ.
type Value struct {
	Text   string
	Exists bool
	Kind   uint32
}

func StringValue(text string) Value { return Value{Text: text, Exists: true, Kind: 1} }

var Current Registry = native{}

type native struct{}

func (native) Get(string) (string, error)                { return "", core.ErrUnsupported }
func (native) Set(string, string) error                  { return core.ErrUnsupported }
func (native) Delete(string) error                       { return core.ErrUnsupported }
func (native) Read(string) (Value, error)                { return Value{}, core.ErrUnsupported }
func (native) CompareAndSwap(string, Value, Value) error { return core.ErrUnsupported }
func (native) Lock() (func(), error)                     { return nil, core.ErrUnsupported }
