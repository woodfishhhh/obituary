package integrate

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
)

// TOMLBlock is a separately parseable, marked section. Owns verifies its decoded
// entries (e.g. strict hook commands or MCP codecs). Refresh must preserve user
// fields/comments and return the desired block; nil permits only equal content.
// Content starts with a table/array-table header so it has an independent scope.
type TOMLBlock struct {
	ID      string
	Content []byte
	Owns    func(map[string]any) bool
	Refresh func(existing []byte) ([]byte, error)
}

var tomlBlockID = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

type tomlBlockState struct {
	data, baseData []byte
	base, prefix   map[string]any
	start, finish  int
	begin, end     string
}

func PlanTOMLBlock(s Scope, path string, spec TOMLBlock, uninstall bool) (Change, error) {
	ch, err := readIntegrationChange(s, path)
	if err != nil {
		return ch, err
	}
	if !tomlBlockID.MatchString(spec.ID) || spec.Owns == nil {
		return ch, fmt.Errorf("TOML block requires an identifier and ownership assertion")
	}
	data := bytes.TrimPrefix(ch.orig, []byte("\xef\xbb\xbf"))
	block, err := inspectTOMLBlock(data, path, spec)
	if err != nil {
		return ch, err
	}
	if uninstall && block.start < 0 {
		return ch, nil
	}
	out, expected := block.baseData, block.base
	if !uninstall {
		body, unchanged, err := block.installBody(spec)
		if err != nil {
			return ch, err
		}
		if unchanged {
			return ch, nil
		}
		out, expected, err = block.replace(spec, body)
		if err != nil {
			return ch, err
		}
	}
	return block.finishChange(ch, spec.ID, out, expected)
}

func inspectTOMLBlock(data []byte, path string, spec TOMLBlock) (tomlBlockState, error) {
	block := tomlBlockState{data: data}
	full, err := decodeTOML(data)
	if err != nil {
		return block, fmt.Errorf("%s: %w", path, err)
	}
	begin, end := "# obit:"+spec.ID+":begin", "# obit:"+spec.ID+":end"
	start, finish, err := tomlMarkedRange(data, begin, end)
	if err != nil {
		return block, err
	}
	baseData := bytes.Clone(data)
	if start >= 0 {
		baseData = append(bytes.Clone(data[:start]), data[finish:]...)
	}
	base, err := decodeTOML(baseData)
	if err != nil {
		return block, fmt.Errorf("removing TOML block changes syntax: %w", err)
	}
	prefix := base
	if start >= 0 {
		prefix, err = decodeTOML(data[:start])
		if err != nil {
			return block, err
		}
		owned, err := decodeTOML(data[start:finish])
		if err != nil || !spec.Owns(owned) {
			return block, fmt.Errorf("%s: edited TOML ownership block; left untouched", path)
		}
		joined, err := mergeTOMLAt(base, owned, prefix)
		if err != nil || !reflect.DeepEqual(full, joined) {
			return block, fmt.Errorf("TOML block shares scope with user configuration; left untouched")
		}
	}
	block.baseData, block.base, block.prefix = baseData, base, prefix
	block.start, block.finish, block.begin, block.end = start, finish, begin, end
	return block, nil
}

func (block tomlBlockState) installBody(spec TOMLBlock) ([]byte, bool, error) {
	body := spec.Content
	if block.start < 0 {
		return []byte(block.begin + "\n" + strings.TrimSpace(string(body)) + "\n" + block.end + "\n"), false, nil
	}
	if spec.Refresh == nil {
		old, _ := decodeTOML(block.data[block.start:block.finish])
		want, err := decodeTOML(spec.Content)
		if err != nil || !reflect.DeepEqual(old, want) {
			return nil, false, fmt.Errorf("TOML update requires a comment-preserving Refresh function")
		}
		return nil, true, nil
	}
	body, err := spec.Refresh(block.data[block.start:block.finish])
	if err != nil {
		return nil, false, err
	}
	// Refresh works on the complete marked block to retain comments.
	if a, b, rangeErr := tomlMarkedRange(body, block.begin, block.end); rangeErr != nil || a != 0 || b != len(body) {
		return nil, false, fmt.Errorf("TOML refresh must retain the complete marked block")
	}
	return body, false, nil
}

func (block tomlBlockState) replace(spec TOMLBlock, body []byte) ([]byte, map[string]any, error) {
	owned, err := decodeTOML(body)
	if err != nil || !spec.Owns(owned) {
		if err == nil {
			return nil, nil, errors.New("proposed TOML block is invalid or unowned: <nil>")
		}
		return nil, nil, fmt.Errorf("proposed TOML block is invalid or unowned: %w", err)
	}
	expected, err := mergeTOMLAt(block.base, owned, block.prefix)
	if err != nil {
		return nil, nil, err
	}
	var out []byte
	if block.start >= 0 {
		out = append(append(bytes.Clone(block.data[:block.start]), body...), block.data[block.finish:]...)
	} else {
		out = append(append(bytes.Clone(block.data), '\n'), body...)
	}
	return out, expected, nil
}

func (block tomlBlockState) finishChange(ch Change, id string, out []byte, expected map[string]any) (Change, error) {
	check, err := decodeTOML(out)
	if err != nil || !reflect.DeepEqual(expected, check) {
		if err == nil {
			return ch, errors.New("proposed TOML block changes user semantics: <nil>")
		}
		return ch, fmt.Errorf("proposed TOML block changes user semantics: %w", err)
	}
	bom := []byte("\xef\xbb\xbf")
	if bytes.HasPrefix(ch.orig, bom) {
		out = append(bytes.Clone(bom), out...)
	}
	ch.Detail = "owned TOML block: " + id
	ch = setIntegrationData(ch, out)
	if ch.Action != "unchanged" {
		if block.start < 0 {
			ch.textEdit = &composeTextEdit{len(ch.orig), len(ch.orig), bytes.Clone(out[len(ch.orig):])}
		} else {
			bomSize := len(ch.orig) - len(block.data)
			// Keep the parser's range. A minimal string diff can move a
			// deletion into the next block's identical "# obit:" header.
			ch.textEdit = &composeTextEdit{block.start + bomSize, block.finish + bomSize, bytes.Clone(out[block.start+bomSize : len(out)-(len(block.data)-block.finish)])}
		}
	}
	return ch, nil
}

func decodeTOML(data []byte) (map[string]any, error) {
	out := map[string]any{}
	err := toml.Unmarshal(data, &out)
	return out, err
}

func mergeTOMLAt(base, add, prefix map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(base)+len(add))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range add {
		old, exists := out[k]
		if !exists {
			out[k] = v
			continue
		}
		x, xok := old.(map[string]any)
		y, yok := v.(map[string]any)
		if xok && yok {
			prior, _ := prefix[k].(map[string]any)
			merged, err := mergeTOMLAt(x, y, prior)
			if err != nil {
				return nil, err
			}
			out[k] = merged
			continue
		}
		a, aok := old.([]any)
		b, bok := v.([]any)
		if aok && bok {
			prior, _ := prefix[k].([]any)
			if len(prior) > len(a) {
				return nil, fmt.Errorf("TOML array %s has inconsistent block scope", k)
			}
			joined := append(append([]any{}, a[:len(prior)]...), b...)
			out[k] = append(joined, a[len(prior):]...)
			continue
		}
		return nil, fmt.Errorf("TOML member %s overlaps user configuration", k)
	}
	return out, nil
}

func tomlMarkedRange(data []byte, begin, end string) (start, finish int, err error) {
	start, finish = -1, -1
	p := unstable.Parser{KeepComments: true}
	p.Reset(data)
	for p.NextExpression() {
		n := p.Expression()
		if n.Kind != unstable.Comment {
			continue
		}
		offset := int(n.Raw.Offset)
		if offset > 0 && data[offset-1] != '\n' {
			continue
		}
		switch strings.TrimSuffix(string(p.Raw(n.Raw)), "\r") {
		case begin:
			if start >= 0 || finish >= 0 {
				return -1, -1, fmt.Errorf("duplicate TOML ownership marker")
			}
			start = offset
			// As in mcpBlockRange, the installer owns the extra separator it
			// always appends before the block, including after an EOF comment.
			if start > 0 && data[start-1] == '\n' {
				start--
				if start > 0 && data[start-1] == '\r' {
					start--
				}
			}
		case end:
			if start < 0 || finish >= 0 {
				return -1, -1, fmt.Errorf("unpaired TOML ownership marker")
			}
			finish = offset + int(n.Raw.Length)
			if finish < len(data) && data[finish] == '\r' {
				finish++
			}
			if finish < len(data) && data[finish] == '\n' {
				finish++
			}
		}
	}
	if err := p.Error(); err != nil {
		return -1, -1, err
	}
	if start >= 0 && finish < 0 {
		return -1, -1, fmt.Errorf("unclosed TOML ownership block")
	}
	return start, finish, nil
}

// RefreshTOMLString updates a direct string member in every matching table or
// array-table using go-toml's AST ranges. Everything outside each literal stays
// byte-for-byte unchanged. It is suitable for a TOMLBlock.Refresh callback.
func RefreshTOMLString(data []byte, table []string, key, value string) ([]byte, error) {
	if _, err := decodeTOML(data); err != nil {
		return nil, err
	}
	encoded, err := toml.Marshal(map[string]string{key: value})
	if err != nil {
		return nil, err
	}
	_, literal, _ := strings.Cut(string(encoded), "=")
	literal = strings.TrimSpace(literal)
	p := unstable.Parser{}
	p.Reset(data)
	var current []string
	var ranges []unstable.Range
	for p.NextExpression() {
		n := p.Expression()
		if n.Kind == unstable.Table || n.Kind == unstable.ArrayTable {
			current = nil
			it := n.Key()
			for it.Next() {
				current = append(current, string(it.Node().Data))
			}
		}
		if n.Kind != unstable.KeyValue || !reflect.DeepEqual(table, current) {
			continue
		}
		it := n.Key()
		if it.Next() && string(it.Node().Data) == key && !it.Next() && n.Value().Kind == unstable.String {
			ranges = append(ranges, n.Value().Raw)
		}
	}
	if err := p.Error(); err != nil {
		return nil, err
	}
	if len(ranges) == 0 {
		return nil, fmt.Errorf("TOML member %s has no directly editable string", key)
	}
	out := bytes.Clone(data)
	for i := len(ranges) - 1; i >= 0; i-- {
		r := ranges[i]
		out = append(append(bytes.Clone(out[:r.Offset]), literal...), out[r.Offset+r.Length:]...)
	}
	return out, nil
}
