package integrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/tailscale/hujson"
)

func parseJSONC(data []byte) (hujson.Value, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if len(bytes.TrimSpace(data)) == 0 {
		data = []byte("{}")
	}
	data = bytes.Clone(data)
	// hujson requires a line terminator for a final // comment.
	if !bytes.HasSuffix(data, []byte("\n")) {
		data = append(data, '\n')
	}
	v, err := hujson.Parse(data)
	if err != nil {
		return v, err
	}
	// Ambiguous members cannot safely be addressed by JSON Pointer.
	for node := range v.All() {
		obj, ok := node.Value.(*hujson.Object)
		if !ok {
			continue
		}
		if err := uniqueJSONCMembers(obj); err != nil {
			return v, err
		}
	}
	return v, nil
}

func uniqueJSONCMembers(obj *hujson.Object) error {
	seen := map[string]bool{}
	for _, member := range obj.Members {
		var key string
		if err := json.Unmarshal(member.Name.Pack(), &key); err != nil {
			// Name extras can contain comments.
			name := member.Name.Clone()
			name.Standardize()
			if err := json.Unmarshal(name.Pack(), &key); err != nil {
				return err
			}
		}
		if seen[key] {
			return fmt.Errorf("duplicate JSONC member %q (left untouched)", key)
		}
		seen[key] = true
	}
	return nil
}

func jsoncJSON(data []byte) ([]byte, error) {
	v, err := parseJSONC(data)
	if err != nil {
		return nil, err
	}
	v.Standardize()
	raw := v.Pack()
	if _, err := parseObject(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func editJSONC(data []byte, edits []ConfigEdit) ([]byte, error) {
	v, err := parseJSONC(data)
	if err != nil {
		return nil, err
	}
	standard := v.Clone()
	standard.Standardize()
	root, err := parseObject(standard.Pack())
	if err != nil {
		return nil, err
	}
	for _, edit := range edits {
		if err := editObject(root, edit); err != nil {
			return nil, err
		}
	}
	want, _ := root.MarshalJSON()
	patches := jsonDiff("", standard.Pack(), want)
	patch, err := marshal(patches)
	if err != nil {
		return nil, err
	}
	// hujson's RFC 6902 implementation moves surrounding comments on removal.
	// Diff at member/element granularity to retain comments inside user entries.
	if err := v.Patch(patch); err != nil {
		return nil, err
	}
	return v.Pack(), nil
}

func containsConfigJSON(values []json.RawMessage, want json.RawMessage) bool {
	for _, value := range values {
		if sameConfigJSON(value, want) {
			return true
		}
	}
	return false
}

type configPatch struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value,omitempty"`
}

func jsonPointer(parent, key string) string {
	return parent + "/" + strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}

func jsonDiff(path string, old, want []byte) []configPatch {
	if sameConfigJSON(old, want) {
		return nil
	}
	a, ae := parseObject(old)
	b, be := parseObject(want)
	if ae == nil && be == nil {
		var patches []configPatch
		for _, k := range a.keys {
			if _, exists := b.get(k); !exists {
				patches = append(patches, configPatch{Op: "remove", Path: jsonPointer(path, k)})
			}
		}
		for _, k := range b.keys {
			if raw, exists := a.get(k); exists {
				patches = append(patches, jsonDiff(jsonPointer(path, k), raw, b.values[k])...)
			} else {
				patches = append(patches, configPatch{Op: "add", Path: jsonPointer(path, k), Value: b.values[k]})
			}
		}
		return patches
	}
	var x, y []json.RawMessage
	if bytes.HasPrefix(bytes.TrimSpace(old), []byte("[")) && bytes.HasPrefix(bytes.TrimSpace(want), []byte("[")) && json.Unmarshal(old, &x) == nil && json.Unmarshal(want, &y) == nil {
		return jsonArrayDiff(path, x, y)
	}
	return []configPatch{{Op: "replace", Path: path, Value: want}}
}

func jsonArrayDiff(path string, x, y []json.RawMessage) []configPatch {
	var patches []configPatch
	i, j := 0, 0
	for i < len(x) || j < len(y) {
		p := jsonPointer(path, strconv.Itoa(j))
		switch {
		case j == len(y):
			patches = append(patches, configPatch{Op: "remove", Path: p})
			i++
		case i == len(x):
			patches = append(patches, configPatch{Op: "add", Path: p, Value: y[j]})
			j++
		case sameConfigJSON(x[i], y[j]):
			i++
			j++
		case containsConfigJSON(x[i+1:], y[j]):
			patches = append(patches, configPatch{Op: "remove", Path: p})
			i++
		case containsConfigJSON(y[j+1:], x[i]):
			patches = append(patches, configPatch{Op: "add", Path: p, Value: y[j]})
			j++
		default:
			patches = append(patches, jsonDiff(p, x[i], y[j])...)
			i++
			j++
		}
	}
	return patches
}
