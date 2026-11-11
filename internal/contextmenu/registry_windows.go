package contextmenu

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/woodfishhhh/obituary/internal/regtxn"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// WindowsRegistry uses x/sys's registry APIs; it never opens HKLM.
type WindowsRegistry struct{ Root string }

func UserRegistry() Registry { return WindowsRegistry{Root: `Software\Classes`} }

func sortedKeys[V any](m map[string]V) []string { return slices.Sorted(maps.Keys(m)) }

func (r WindowsRegistry) ReadTree(path string) (Tree, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, r.Root+`\`+path, registry.READ)
	if errors.Is(err, registry.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer key.Close()
	tree := Tree{}
	err = readKey(key, "", tree)
	return tree, err
}

func readKey(key registry.Key, path string, tree Tree) error {
	return readKeyWith(key, path, tree, registry.OpenKey)
}

func readKeyWith(key registry.Key, path string, tree Tree, open func(registry.Key, string, uint32) (registry.Key, error)) error {
	names, err := key.ReadValueNames(-1)
	if err != nil {
		return err
	}
	tree[path] = map[string]string{}
	for _, name := range names {
		value, kind, err := key.GetStringValue(name)
		if err != nil {
			return fmt.Errorf("preserve value %s: %w", name, err)
		}
		if kind != registry.SZ {
			return fmt.Errorf("preserve non-REG_SZ value %s", name)
		}
		tree[path][name] = value
	}
	children, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return err
	}
	for _, name := range children {
		child, err := open(key, name, registry.READ)
		if err != nil {
			return err
		}
		rel := name
		if path != "" {
			rel = path + `\` + name
		}
		err = readKeyWith(child, rel, tree, open)
		child.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r WindowsRegistry) WriteTree(path string, tree Tree) error {
	for _, rel := range sortedKeys(tree) {
		key, _, err := registry.CreateKey(registry.CURRENT_USER, r.Root+`\`+path+keySuffix(rel), registry.SET_VALUE)
		if err != nil {
			return err
		}
		err = writeValues(key, tree[rel])
		key.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func writeValues(key registry.Key, values map[string]string) error {
	for _, name := range sortedKeys(values) {
		if err := key.SetStringValue(name, values[name]); err != nil {
			return err
		}
	}
	return nil
}

func (r WindowsRegistry) Lock() (func(), error) {
	return regtxn.Lock("contextmenu:" + strings.ToUpper(r.Root))
}

func (r WindowsRegistry) CompareAndSwap(c Change) error {
	return regtxn.Do(func(tx *regtxn.Transaction) error {
		key, err := tx.OpenPinned(registry.CURRENT_USER, r.Root+`\`+c.Path, registry.READ|registry.WRITE|windows.DELETE)
		if errors.Is(err, registry.ErrNotExist) && c.Before == nil {
			if c.After == nil {
				return nil
			}
			key, err = tx.CreateNew(registry.CURRENT_USER, r.Root+`\`+c.Path, registry.READ|registry.WRITE|windows.DELETE)
		} else if err == nil {
			current := Tree{}
			err = readKeyWith(key, "", current, tx.OpenPinned)
			if err == nil && !reflect.DeepEqual(current, c.Before) {
				err = fmt.Errorf("preserved %s: registry changed after planning", c.Path)
			}
		}
		if key != 0 {
			defer key.Close()
		}
		if err != nil {
			return err
		}
		if c.After == nil {
			return deleteTree(tx, key, c.Before)
		}
		return writeTree(tx, key, c.After)
	})
}

func writeTree(tx *regtxn.Transaction, root registry.Key, tree Tree) error {
	for _, rel := range sortedKeys(tree) {
		key, err := tx.Create(root, rel, registry.READ|registry.WRITE)
		if err != nil {
			return err
		}
		err = writeValues(key, tree[rel])
		key.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteTree(tx *regtxn.Transaction, root registry.Key, tree Tree) error {
	keys := sortedKeys(tree)
	slices.Reverse(keys)
	for _, rel := range keys {
		if err := deleteCheckedKey(tx, root, rel, tree[rel]); err != nil {
			return err
		}
	}
	return nil
}

func deleteCheckedKey(tx *regtxn.Transaction, root registry.Key, rel string, expected map[string]string) error {
	key, err := tx.OpenPinned(root, rel, registry.READ|windows.DELETE)
	if err != nil {
		return err
	}
	defer key.Close()
	current := Tree{}
	if err := readKeyWith(key, "", current, tx.OpenPinned); err != nil {
		return err
	}
	if !reflect.DeepEqual(current, Tree{"": expected}) {
		return fmt.Errorf("preserved %s: registry changed before deletion", rel)
	}
	return tx.Delete(key)
}

func keySuffix(path string) string {
	if path == "" {
		return ""
	}
	return `\` + path
}
