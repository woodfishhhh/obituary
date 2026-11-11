package integrate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
)

// ConfigFormat is explicit: a JSON5 file never falls back to a JSON writer.
type ConfigFormat string

const (
	ConfigJSON  ConfigFormat = "json"
	ConfigJSONC ConfigFormat = "jsonc"
	ConfigJSON5 ConfigFormat = "json5"
	ConfigYAML  ConfigFormat = "yaml"
)

// ConfigEdit changes one object member. Remove leaves enclosing objects intact.
// Value is strict JSON; nil is not a deletion. Expected checks the decoded value
// before editing when CheckExpected is set (nil then means the key is absent).
type ConfigEdit struct {
	Path          []string
	Value         json.RawMessage
	Remove        bool
	CheckExpected bool
	Expected      json.RawMessage
}

// PlanConfigEdits is a low-level format-preserving editor. Callers must establish
// ownership before proposing edits; MCP and hook planners do that themselves.
func PlanConfigEdits(s Scope, path string, format ConfigFormat, edits []ConfigEdit) (Change, error) {
	ch, err := readIntegrationChange(s, path)
	if err != nil {
		return ch, err
	}
	data, err := editConfig(ch.orig, format, edits)
	if err != nil {
		return ch, fmt.Errorf("%s: %w", path, err)
	}
	return setIntegrationData(ch, data), nil
}

func editConfig(data []byte, format ConfigFormat, edits []ConfigEdit) ([]byte, error) {
	if format == ConfigJSON5 {
		return nil, fmt.Errorf("JSON5 automatic editing is unsupported; manually apply these object-member edits in the host configuration editor, retaining comments: %s", manualConfigEdits(edits))
	}
	before, err := configJSON(data, format)
	if err != nil {
		return nil, err
	}
	root, err := parseObject(before)
	if err != nil {
		return nil, err
	}
	for _, edit := range edits {
		if err := editObject(root, edit); err != nil {
			return nil, err
		}
	}
	want, err := root.MarshalJSON()
	if err != nil {
		return nil, err
	}
	if sameConfigJSON(before, want) {
		return bytes.Clone(data), nil
	}
	var out []byte
	switch format {
	case ConfigJSON:
		out, err = root.pretty()
	case ConfigJSONC:
		out, err = editJSONC(data, edits)
	case ConfigYAML:
		out, err = editYAML(data, edits)
	default:
		return nil, fmt.Errorf("unsupported configuration format %q", format)
	}
	if err != nil {
		return nil, err
	}
	check, err := configJSON(out, format)
	if err != nil || !sameConfigJSON(want, check) {
		if err == nil {
			return nil, errors.New("configuration edit changes unrelated semantics (left untouched): <nil>")
		}
		return nil, fmt.Errorf("configuration edit changes unrelated semantics (left untouched): %w", err)
	}
	if bytes.HasPrefix(data, []byte("\xef\xbb\xbf")) && !bytes.HasPrefix(out, []byte("\xef\xbb\xbf")) {
		out = append([]byte("\xef\xbb\xbf"), out...)
	}
	return out, nil
}

func configJSON(data []byte, format ConfigFormat) ([]byte, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if len(bytes.TrimSpace(data)) == 0 {
		data = []byte("{}")
	}
	switch format {
	case ConfigJSON:
		root, err := parseObject(data)
		if err != nil {
			return nil, err
		}
		return root.MarshalJSON()
	case ConfigJSONC:
		return jsoncJSON(data)
	case ConfigYAML:
		return yamlJSON(data)
	case ConfigJSON5:
		return nil, fmt.Errorf("JSON5 automatic editing is unsupported; use the host configuration editor")
	default:
		return nil, fmt.Errorf("unsupported configuration format %q", format)
	}
}

func editObject(root *object, edit ConfigEdit) error {
	if len(edit.Path) == 0 {
		return fmt.Errorf("configuration edit needs a member path")
	}
	key := edit.Path[0]
	old, exists := root.get(key)
	if len(edit.Path) > 1 {
		return editObjectChild(root, edit, old, exists)
	}
	if edit.CheckExpected && ((edit.Expected == nil) != !exists || exists && !sameConfigJSON(old, edit.Expected)) {
		return fmt.Errorf("%s changed from the expected configuration", key)
	}
	if edit.Remove {
		root.remove(key)
		return nil
	}
	if !json.Valid(edit.Value) {
		return fmt.Errorf("%s: replacement is not valid JSON", key)
	}
	return root.set(key, edit.Value)
}

func editObjectChild(root *object, edit ConfigEdit, old json.RawMessage, exists bool) error {
	key := edit.Path[0]
	if !exists && edit.Remove {
		if edit.CheckExpected && edit.Expected != nil {
			return fmt.Errorf("%s changed from the expected configuration", key)
		}
		return nil
	}
	child, err := parseObject(old)
	if err != nil {
		return fmt.Errorf("%s is not an object: %w", key, err)
	}
	edit.Path = edit.Path[1:]
	if err := editObject(child, edit); err != nil {
		return err
	}
	return root.set(key, child)
}

// Keep JSON numbers exact when validating a format conversion.
func sameConfigJSON(a, b []byte) bool {
	decode := func(data []byte) (any, error) {
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		var value any
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
			if err == nil {
				return nil, errors.New("trailing JSON data: <nil>")
			}
			return nil, fmt.Errorf("trailing JSON data: %w", err)
		}
		return value, nil
	}
	x, err := decode(a)
	if err != nil {
		return false
	}
	y, err := decode(b)
	return err == nil && reflect.DeepEqual(x, y)
}

func manualConfigEdits(edits []ConfigEdit) string {
	var steps []string
	for _, e := range edits {
		path, _ := marshal(e.Path)
		if e.Remove {
			steps = append(steps, "remove member "+string(path)+" only after verifying Obituary ownership")
		} else {
			steps = append(steps, "set member "+string(path)+" to "+string(e.Value)+" only if absent or owned by Obituary")
		}
	}
	return strings.Join(steps, "; ")
}
