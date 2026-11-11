package integrate

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"go.yaml.in/yaml/v3"
)

func parseYAML(data []byte) (*yaml.Node, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if len(bytes.TrimSpace(data)) == 0 {
		data = []byte("{}")
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("expected one YAML document: <nil>")
		}
		return nil, fmt.Errorf("expected one YAML document: %w", err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("configuration must be a YAML mapping")
	}
	var before any
	if err := doc.Decode(&before); err != nil {
		return nil, err
	}
	// Node preserves order, comments, tags and style. Confirm the library's
	// round trip before editing, including YAML-specific scalar semantics.
	encoded, err := yaml.Marshal(&doc)
	if err != nil {
		return nil, err
	}
	var after any
	if err := yaml.Unmarshal(encoded, &after); err != nil || !reflect.DeepEqual(before, after) {
		if err == nil {
			return nil, errors.New("YAML round trip changes semantics: <nil>")
		}
		return nil, fmt.Errorf("YAML round trip changes semantics: %w", err)
	}
	return &doc, nil
}

func yamlJSON(data []byte) ([]byte, error) {
	doc, err := parseYAML(data)
	if err != nil {
		return nil, err
	}
	return yamlNodeJSON(doc.Content[0])
}

func yamlNodeJSON(n *yaml.Node) ([]byte, error) {
	var value any
	if err := n.Decode(&value); err != nil {
		return nil, err
	}
	return marshal(value)
}

func editYAML(data []byte, edits []ConfigEdit) ([]byte, error) {
	doc, err := parseYAML(data)
	if err != nil {
		return nil, err
	}
	raw, err := yamlNodeJSON(doc.Content[0])
	if err != nil {
		return nil, err
	}
	root, err := parseObject(raw)
	if err != nil {
		return nil, err
	}
	for _, edit := range edits {
		if err := editObject(root, edit); err != nil {
			return nil, err
		}
	}
	want, _ := root.MarshalJSON()
	var target yaml.Node
	if err := yaml.Unmarshal(want, &target); err != nil {
		return nil, err
	}
	if err := syncYAML(doc.Content[0], target.Content[0]); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func sameYAML(a, b *yaml.Node) bool {
	x, xe := yamlNodeJSON(a)
	y, ye := yamlNodeJSON(b)
	return xe == nil && ye == nil && sameConfigJSON(x, y)
}

func containsYAML(values []*yaml.Node, want *yaml.Node) bool {
	for _, value := range values {
		if sameYAML(value, want) {
			return true
		}
	}
	return false
}

func syncYAML(dst, src *yaml.Node) error {
	if sameYAML(dst, src) {
		return nil
	}
	if dst.Anchor != "" || dst.Kind == yaml.AliasNode {
		return fmt.Errorf("edited YAML member uses an anchor or alias; left untouched")
	}
	if dst.Kind == yaml.MappingNode && src.Kind == yaml.MappingNode {
		return syncYAMLMapping(dst, src)
	}
	if dst.Kind == yaml.SequenceNode && src.Kind == yaml.SequenceNode {
		return syncYAMLSequence(dst, src)
	}
	before, line, after := dst.HeadComment, dst.LineComment, dst.FootComment
	plainYAML(src)
	*dst = *src
	dst.HeadComment, dst.LineComment, dst.FootComment = before, line, after
	return nil
}

func syncYAMLMapping(dst, src *yaml.Node) error {
	for _, key := range dst.Content {
		if key.Tag == "!!merge" {
			return fmt.Errorf("edited YAML mapping uses a merge key; left untouched")
		}
	}
	wanted := map[string]*yaml.Node{}
	for i := 0; i < len(src.Content); i += 2 {
		wanted[src.Content[i].Value] = src.Content[i+1]
	}
	var kept []*yaml.Node
	for i := 0; i < len(dst.Content); i += 2 {
		key, value := dst.Content[i], dst.Content[i+1]
		next, ok := wanted[key.Value]
		if !ok {
			keepYAMLComments(dst, key, value)
			continue
		}
		if err := syncYAML(value, next); err != nil {
			return err
		}
		kept = append(kept, key, value)
		delete(wanted, key.Value)
	}
	for i := 0; i < len(src.Content); i += 2 {
		if _, ok := wanted[src.Content[i].Value]; ok {
			plainYAML(src.Content[i])
			plainYAML(src.Content[i+1])
			kept = append(kept, src.Content[i], src.Content[i+1])
		}
	}
	dst.Content = kept
	return nil
}

func syncYAMLSequence(dst, src *yaml.Node) error {
	var kept []*yaml.Node
	x, y := dst.Content, src.Content
	for i, j := 0, 0; i < len(x) || j < len(y); {
		switch {
		case j == len(y):
			keepYAMLComments(dst, x[i])
			i++
		case i == len(x):
			plainYAML(y[j])
			kept = append(kept, y[j])
			j++
		case sameYAML(x[i], y[j]):
			kept = append(kept, x[i])
			i++
			j++
		case containsYAML(x[i+1:], y[j]):
			keepYAMLComments(dst, x[i])
			i++
		case containsYAML(y[j+1:], x[i]):
			plainYAML(y[j])
			kept = append(kept, y[j])
			j++
		default:
			if err := syncYAML(x[i], y[j]); err != nil {
				return err
			}
			kept = append(kept, x[i])
			i++
			j++
		}
	}
	dst.Content = kept
	return nil
}

func plainYAML(n *yaml.Node) {
	// JSON input has flow/quoted styles; new YAML nodes use the host format.
	n.Style = 0
	for _, child := range n.Content {
		plainYAML(child)
	}
}

func keepYAMLComments(parent *yaml.Node, nodes ...*yaml.Node) {
	for _, n := range nodes {
		for _, comment := range []string{n.HeadComment, n.LineComment, n.FootComment} {
			if comment != "" {
				parent.FootComment = strings.TrimSpace(parent.FootComment + "\n" + comment)
			}
		}
		keepYAMLComments(parent, n.Content...)
	}
}
