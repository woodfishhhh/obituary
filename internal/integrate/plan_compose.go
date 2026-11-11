package integrate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2/unstable"
	"github.com/tailscale/hujson"
	"go.yaml.in/yaml/v3"
)

// ComposeAgentChanges combines plans computed from the same original file before
// Apply. JSON/JSONC, YAML and TOML are selected by filename. Arrays are atomic:
// differing edits to one array, ancestor/child edits and ambiguous text edits
// are conflicts. Identical edits and managed owner intentions are combined;
// host-command changes stay opaque.
// Combined extension changes join the required group so a shared file is written
// once; Implicit remains true only when every contributing change was implicit.
func ComposeAgentChanges(changes []Change) ([]Change, error) {
	out, err := composeOwnerChanges(changes)
	if err != nil {
		return nil, err
	}
	groups, order := composePathGroups(out)
	removed := map[int]bool{}
	for _, key := range order {
		indices := groups[key]
		if len(indices) < 2 {
			continue
		}
		group := make([]Change, 0, len(indices))
		for _, i := range indices {
			group = append(group, out[i])
		}
		ch, err := composeFileChanges(group)
		if err != nil {
			return nil, fmt.Errorf("compose %s: %w", group[0].Path, err)
		}
		written := false
		for _, i := range indices {
			if out[i].Action == "preserve" && len(out[i].preservePath) > 0 {
				continue
			}
			if !written {
				out[i], written = ch, true
			} else {
				removed[i] = true
			}
		}
	}
	result := make([]Change, 0, len(out))
	readOnly := slices.ContainsFunc(changes, func(c Change) bool { return c.readOnly })
	for i, ch := range out {
		if !removed[i] {
			ch.readOnly = ch.readOnly || readOnly
			result = append(result, ch)
		}
	}
	bindSharedTransactionGroups(result)
	return result, nil
}

type plannedValue struct {
	path   []string
	value  any
	remove bool
}

func composeFileChanges(changes []Change) (Change, error) {
	first := changes[0]
	if strings.EqualFold(filepath.Ext(first.Path), ".md") {
		return composeRuleChanges(changes)
	}
	format, err := composeFormat(first.Path)
	if err != nil {
		return first, err
	}
	base, err := composeDecode(first.orig, format)
	if err != nil {
		return first, err
	}
	p := composedConfig{first: first, format: format, base: base, sources: [][]byte{first.orig}}
	for _, ch := range changes {
		if err := p.add(ch, changes); err != nil {
			return p.first, err
		}
	}
	return p.finish()
}

type composedConfig struct {
	first     Change
	format    ConfigFormat
	base      map[string]any
	edits     []plannedValue
	textEdits []composeTextEdit
	sources   [][]byte
}

func mergeChangeMetadata(first, ch Change) Change {
	if ch.Extension != first.Extension {
		first.Extension = ""
	}
	first.Implicit = first.Implicit && ch.Implicit
	if ch.phase < 0 || first.phase < 0 {
		first.phase = min(first.phase, ch.phase)
	} else {
		first.phase = max(first.phase, ch.phase)
	}
	return first
}

func (p *composedConfig) add(ch Change, changes []Change) error {
	if ch.existed != p.first.existed || !bytes.Equal(ch.orig, p.first.orig) || ch.project != p.first.project {
		return fmt.Errorf("plans have different original content or boundaries")
	}
	if ch.next != nil || ch.claude != nil || len(ch.Commands) != 0 || ch.delete || ch.Action == "preserve" && len(ch.preservePath) == 0 {
		return fmt.Errorf("opaque, preserved or whole-file removal plans cannot be combined")
	}
	p.first = mergeChangeMetadata(p.first, ch)
	if ch.Action == "unchanged" || ch.Action == "preserve" {
		return nil
	}
	if ch.Action != "create" && ch.Action != "update" {
		return fmt.Errorf("unsupported action %q", ch.Action)
	}
	want, err := composeDecode(ch.data, p.format)
	if err != nil {
		return fmt.Errorf("invalid planned configuration: %w", err)
	}
	current := composeDiff(nil, p.base, want)
	unique, err := uniqueConfigEdits(current, p.edits, changes)
	if err != nil {
		return err
	}
	if len(current) == 0 && !bytes.Equal(ch.orig, ch.data) {
		return fmt.Errorf("format-only edits require separate review")
	}
	p.edits = append(p.edits, unique...)
	if !slices.ContainsFunc(p.sources, func(source []byte) bool { return bytes.Equal(source, ch.data) }) {
		p.sources = append(p.sources, ch.data)
	}
	if p.format == composeTOML && len(unique) > 0 {
		edit := composeTextDiff(ch.orig, ch.data)
		if ch.textEdit != nil {
			edit = *ch.textEdit
		}
		p.textEdits = append(p.textEdits, edit)
	}
	return nil
}

func uniqueConfigEdits(current, prior []plannedValue, changes []Change) ([]plannedValue, error) {
	var unique []plannedValue
	for _, edit := range current {
		duplicate, err := duplicateConfigEdit(edit, prior)
		if err != nil {
			return nil, err
		}
		for _, preserved := range changes {
			if preserved.Action == "preserve" && composePathOverlaps(edit.path, preserved.preservePath) {
				return nil, fmt.Errorf("edit overlaps preserved member %q", preserved.preservePath)
			}
		}
		if !duplicate {
			unique = append(unique, edit)
		}
	}
	return unique, nil
}

func duplicateConfigEdit(edit plannedValue, prior []plannedValue) (bool, error) {
	for _, previous := range prior {
		if !composePathOverlaps(edit.path, previous.path) {
			continue
		}
		if slices.Equal(edit.path, previous.path) && edit.remove == previous.remove && reflect.DeepEqual(edit.value, previous.value) {
			return true, nil
		}
		return false, fmt.Errorf("overlapping edits at %q and %q", edit.path, previous.path)
	}
	return false, nil
}

func (p *composedConfig) finish() (Change, error) {
	first := p.first
	if len(p.edits) == 0 {
		first.Action, first.data = "unchanged", nil
		return first, nil
	}
	expected, err := composeDecode(first.orig, p.format)
	if err != nil {
		return first, err
	}
	var configEdits []ConfigEdit
	for _, edit := range p.edits {
		composeSet(expected, edit)
		if p.format == composeTOML {
			continue
		}
		value, err := marshal(edit.value)
		if err != nil {
			return first, err
		}
		configEdits = append(configEdits, ConfigEdit{Path: edit.path, Value: json.RawMessage(value), Remove: edit.remove})
	}
	var data []byte
	if p.format == composeTOML {
		data, err = composeTOMLText(first.orig, p.textEdits)
	} else {
		data, err = editConfig(first.orig, p.format, configEdits)
	}
	if err != nil {
		return first, err
	}
	actual, err := composeDecode(data, p.format)
	if err != nil || !reflect.DeepEqual(expected, actual) {
		if err == nil {
			return first, errors.New("combined plan changes unrelated configuration semantics: <nil>")
		}
		return first, fmt.Errorf("combined plan changes unrelated configuration semantics: %w", err)
	}
	if bytes.HasPrefix(first.orig, []byte("\xef\xbb\xbf")) != bytes.HasPrefix(data, []byte("\xef\xbb\xbf")) {
		return first, fmt.Errorf("combined plan changes the BOM")
	}
	if err := composeCheckComments(p.sources, data, p.format); err != nil {
		return first, err
	}
	first.Action, first.data = "unchanged", nil
	first = setIntegrationData(first, data)
	first.Detail = "combined non-overlapping configuration changes"
	return first, nil
}

func composeDiff(path []string, before, after map[string]any) []plannedValue {
	keys := make([]string, 0, len(before)+len(after))
	for key := range before {
		keys = append(keys, key)
	}
	for key := range after {
		if _, ok := before[key]; !ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var result []plannedValue
	for _, key := range keys {
		old, existed := before[key]
		want, exists := after[key]
		if existed == exists && reflect.DeepEqual(old, want) {
			continue
		}
		p := append(slices.Clone(path), key)
		a, aok := old.(map[string]any)
		b, bok := want.(map[string]any)
		if exists && bok && len(b) > 0 && (aok || !existed) {
			result = append(result, composeDiff(p, a, b)...)
		} else {
			result = append(result, plannedValue{path: p, value: want, remove: !exists})
		}
	}
	return result
}

func composePathOverlaps(a, b []string) bool {
	return slices.Equal(a[:min(len(a), len(b))], b[:min(len(a), len(b))])
}

const composeTOML ConfigFormat = "toml"

func composeFormat(path string) (ConfigFormat, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json", ".jsonc":
		return ConfigJSONC, nil
	case ".yaml", ".yml":
		return ConfigYAML, nil
	case ".toml":
		return composeTOML, nil
	default:
		return "", fmt.Errorf("multiple plans for an unsupported file format")
	}
}

func composeDecode(data []byte, format ConfigFormat) (map[string]any, error) {
	if format == composeTOML {
		return decodeTOML(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")))
	}
	raw, err := configJSON(data, format)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	out := map[string]any{}
	err = dec.Decode(&out)
	return out, err
}

type composeTextEdit struct {
	start, end int
	data       []byte
}

// TOML's maintained AST edits already preserve byte ranges. Compose conservative
// single-span replacements without reserializing TOML (which would drop comments).
// Disjoint key changes whose text spans overlap are intentionally rejected.
func composeTextDiff(old, want []byte) composeTextEdit {
	start := 0
	for start < len(old) && start < len(want) && old[start] == want[start] {
		start++
	}
	end, last := len(old), len(want)
	for end > start && last > start && old[end-1] == want[last-1] {
		end--
		last--
	}
	return composeTextEdit{start, end, bytes.Clone(want[start:last])}
}

func composeTOMLText(original []byte, edits []composeTextEdit) ([]byte, error) {
	slices.SortStableFunc(edits, func(a, b composeTextEdit) int { return a.start - b.start })
	var out []byte
	cursor := 0
	for _, edit := range edits {
		if edit.start < cursor {
			return nil, fmt.Errorf("TOML text edits overlap; use independent blocks")
		}
		out = append(out, original[cursor:edit.start]...)
		out = append(out, edit.data...)
		cursor = edit.end
	}
	return append(out, original[cursor:]...), nil
}

// Parser-provided comment nodes are inspected, so strings containing comment
// delimiters cannot masquerade as preserved comments. The small Extra regex only
// splits hujson's already-tokenized whitespace/comment fragments.
var composeJSONComment = regexp.MustCompile(`(?s)//[^\r\n]*|/\*.*?\*/`)
var composeOwnedMarker = regexp.MustCompile(`^# obit:[a-z][a-z0-9-]*:(begin|end)( \(managed by obit init\))?$`)

func composeComments(data []byte, format ConfigFormat) (map[string]int, error) {
	out := map[string]int{}
	add := func(comment string) {
		comment = strings.TrimSpace(comment)
		if comment != "" && !composeOwnedMarker.MatchString(comment) {
			out[comment]++
		}
	}
	var err error
	switch format {
	case ConfigJSONC:
		err = composeJSONComments(data, add)
	case ConfigYAML:
		err = composeYAMLComments(data, add)
	case composeTOML:
		err = composeTOMLComments(data, add)
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func composeJSONComments(data []byte, add func(string)) error {
	v, err := parseJSONC(data)
	if err != nil {
		return err
	}
	extra := func(data []byte) {
		for _, comment := range composeJSONComment.FindAll(data, -1) {
			add(string(comment))
		}
	}
	for n := range v.All() {
		extra(n.BeforeExtra)
		extra(n.AfterExtra)
		switch node := n.Value.(type) {
		case *hujson.Object:
			extra(node.AfterExtra)
		case *hujson.Array:
			extra(node.AfterExtra)
		}
	}
	return nil
}

func composeYAMLComments(data []byte, add func(string)) error {
	var node yaml.Node
	if err := yaml.Unmarshal(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), &node); err != nil {
		return err
	}
	var visit func(*yaml.Node)
	visit = func(n *yaml.Node) {
		for _, comment := range []string{n.HeadComment, n.LineComment, n.FootComment} {
			for _, line := range strings.Split(comment, "\n") {
				add(line)
			}
		}
		for _, child := range n.Content {
			visit(child)
		}
	}
	visit(&node)
	return nil
}

func composeTOMLComments(data []byte, add func(string)) error {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	parser := unstable.Parser{KeepComments: true}
	parser.Reset(data)
	var visit func(*unstable.Node)
	visit = func(n *unstable.Node) {
		if n.Kind == unstable.Comment {
			add(string(parser.Raw(n.Raw)))
		}
		children := n.Children()
		for children.Next() {
			visit(children.Node())
		}
	}
	for parser.NextExpression() {
		visit(parser.Expression())
	}
	return parser.Error()
}

func composeCheckComments(sources [][]byte, out []byte, format ConfigFormat) error {
	base, err := composeComments(sources[0], format)
	if err != nil {
		return err
	}
	expected := map[string]int{}
	for comment, count := range base {
		expected[comment] = count
	}
	for _, source := range sources[1:] {
		comments, err := composeComments(source, format)
		if err != nil {
			return err
		}
		for comment, count := range comments {
			expected[comment] += max(0, count-base[comment])
		}
	}
	actual, err := composeComments(out, format)
	if err != nil {
		return err
	}
	comments := make([]string, 0, len(expected))
	for comment := range expected {
		comments = append(comments, comment)
	}
	slices.Sort(comments)
	for _, comment := range comments {
		count := expected[comment]
		if actual[comment] < count {
			return fmt.Errorf("combined plan would lose comment %q", comment)
		}
	}
	return nil
}

func composeSet(root map[string]any, edit plannedValue) {
	for _, key := range edit.path[:len(edit.path)-1] {
		child, ok := root[key].(map[string]any)
		if !ok {
			child = map[string]any{}
			root[key] = child
		}
		root = child
	}
	key := edit.path[len(edit.path)-1]
	if edit.remove {
		delete(root, key)
	} else {
		root[key] = edit.value
	}
}

func composePathGroups(out []Change) (map[string][]int, []string) {
	groups := map[string][]int{}
	var order []string
	for i, ch := range out {
		if ch.Manual {
			continue // Multiple instructions can describe the same untouched host file.
		}
		key := strings.ToLower(filepath.Clean(ch.Path))
		if len(groups[key]) == 0 {
			order = append(order, key)
		}
		groups[key] = append(groups[key], i)
	}
	return groups, order
}
