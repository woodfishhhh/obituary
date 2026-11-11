// Package runkey is the sole native boundary for login startup registration.
package runkey

import (
	"errors"
	"fmt"
	"strings"

	"github.com/woodfishhhh/obituary/internal/regtxn"
	"golang.org/x/sys/windows/registry"
)

// path is overridden at link time by subprocess tests, never by user input.
var path = `Software\Microsoft\Windows\CurrentVersion\Run`

// Value distinguishes absent, empty and non-REG_SZ startup values.
type Value struct {
	Text   string
	Exists bool
	Kind   uint32
}

func StringValue(text string) Value { return Value{Text: text, Exists: true, Kind: registry.SZ} }

type Registry interface {
	Get(string) (string, error)
	Read(string) (Value, error)
	CompareAndSwap(string, Value, Value) error
	Lock() (func(), error)
}

// Current is replaced by testenv.Main before any package test can run.
var Current Registry = native{}

type native struct{ root string }

func (r native) keyPath() string {
	if r.root != "" {
		return r.root
	}
	return path
}

func (r native) Get(name string) (string, error) {
	value, err := r.Read(name)
	return value.Text, err
}

func (r native) Read(name string) (Value, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, r.keyPath(), registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return Value{}, nil
	}
	if err != nil {
		return Value{}, err
	}
	defer key.Close()
	return readValue(key, name)
}

func readValue(key registry.Key, name string) (Value, error) {
	value, kind, err := key.GetStringValue(name)
	if errors.Is(err, registry.ErrNotExist) {
		return Value{}, nil
	}
	if err != nil {
		return Value{}, err
	}
	if kind != registry.SZ {
		return Value{}, fmt.Errorf("preserve non-REG_SZ Run value %s", name)
	}
	return StringValue(value), nil
}

func (r native) Lock() (func(), error) { return regtxn.Lock("runkey:" + strings.ToUpper(r.keyPath())) }

func (r native) CompareAndSwap(name string, before, after Value) error {
	return regtxn.Do(func(tx *regtxn.Transaction) error {
		key, err := tx.OpenPinned(registry.CURRENT_USER, r.keyPath(), registry.QUERY_VALUE|registry.SET_VALUE)
		if errors.Is(err, registry.ErrNotExist) && !before.Exists {
			if !after.Exists {
				return nil
			}
			key, err = tx.Create(registry.CURRENT_USER, r.keyPath(), registry.QUERY_VALUE|registry.SET_VALUE)
		}
		if err != nil {
			return err
		}
		defer key.Close()
		if err := tx.PinWrite(key); err != nil {
			return err
		}
		current, err := readValue(key, name)
		if err != nil {
			return err
		}
		if current != before {
			return fmt.Errorf("run value %s changed since planning", name)
		}
		if before == after {
			return nil
		}
		if !after.Exists {
			return key.DeleteValue(name)
		}
		if after.Kind != registry.SZ {
			return fmt.Errorf("only REG_SZ Run values can be written")
		}
		return key.SetStringValue(name, after.Text)
	})
}
