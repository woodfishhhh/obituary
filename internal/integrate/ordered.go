// Package integrate installs and removes Obituary's integrations in agent
// configuration (hooks, instruction files). Installation is idempotent, backs
// up every change, and edits only owned content to preserve user configuration.
package integrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// object is a JSON object that keeps its key order, so rewriting a user's
// settings file only changes what Obituary owns.
//
// Keep this hand-written object rather than a library: encoding/json sorts
// map keys, and sjson v1.2.8 / gjson v1.20.2 keep duplicate fields and act on
// the first one, whereas we keep the first position and the last value (the
// one JavaScript's JSON.parse and encoding/json use). sjson also HTML-escapes
// keys such as "中文&" (sjson.go appendStringify) and has no raw-key or
// EscapeHTML option, so it would still need a custom object writer.
type object struct {
	keys   []string
	values map[string]json.RawMessage
}

func newObject() *object { return &object{values: map[string]json.RawMessage{}} }

func parseObject(data []byte) (*object, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")) // tolerate a UTF-8 BOM
	if len(bytes.TrimSpace(data)) == 0 {
		return newObject(), nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf("not a JSON object")
	}
	o := newObject()
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected token %v", tok)
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		if _, dup := o.values[key]; !dup {
			o.keys = append(o.keys, key)
		}
		o.values[key] = raw
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	// Nothing may follow the object: concatenated objects or a trailing
	// fragment mean the file is not what it seems, and must not be rewritten.
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("unexpected data after the JSON object")
	}
	return o, nil
}

func (o *object) get(key string) (json.RawMessage, bool) {
	v, ok := o.values[key]
	return v, ok
}

func (o *object) set(key string, value any) error {
	raw, ok := value.(json.RawMessage)
	if !ok {
		b, err := marshal(value)
		if err != nil {
			return err
		}
		raw = b
	}
	if _, exists := o.values[key]; !exists {
		o.keys = append(o.keys, key)
	}
	o.values[key] = raw
	return nil
}

func (o *object) remove(key string) {
	if _, ok := o.values[key]; !ok {
		return
	}
	delete(o.values, key)
	for i, k := range o.keys {
		if k == key {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			return
		}
	}
}

func (o *object) empty() bool { return len(o.keys) == 0 }

func (o *object) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		key, _ := marshal(k)
		b.Write(key)
		b.WriteByte(':')
		b.Write(o.values[k])
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// pretty renders o with two-space indentation and a trailing newline.
func (o *object) pretty() ([]byte, error) {
	raw, err := o.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

// marshal is json.Marshal without HTML escaping: re-serialising a user's
// settings must not turn their "&&" into its & escape.
func marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}
