package integrate

import (
	"bytes"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2/unstable"
	"github.com/woodfishhhh/obituary/internal/config"
)

// PlanGraveyardConfig uses the same original-check, backup and atomic Apply
// path as PlanCodexWindowsShell. go-toml's AST supplies byte ranges, preserving
// comments and unknown keys; re-encoding Config would discard both.
func PlanGraveyardConfig(path, mode string, retentionDays int) (Change, error) {
	ch, err := readIntegrationChange(Scope{Global: true}, path)
	if err != nil {
		return ch, err
	}
	if _, err := config.Parse(ch.orig, path); err != nil {
		return ch, err
	}
	data := ch.orig
	if !ch.existed {
		data = []byte(config.Template)
	}
	data, err = setConfigValue(data, "graveyard", "mode", strconv.Quote(mode))
	if err != nil {
		return ch, err
	}
	data, err = setConfigValue(data, "graveyard", "retention_days", strconv.Itoa(retentionDays))
	if err != nil {
		return ch, err
	}
	if _, err := config.Parse(data, path); err != nil {
		return ch, err
	}
	ch.Detail = fmt.Sprintf("graveyard.mode=%q; graveyard.retention_days=%d", mode, retentionDays)
	return setIntegrationData(ch, data), nil
}

func tomlKeys(n *unstable.Node) []string {
	var keys []string
	iter := n.Key()
	for iter.Next() {
		// Match go-toml/v2 unmarshaler.go structPlan.lookupFold, used by
		// config.Parse; preserve the original spelling in the source bytes.
		keys = append(keys, strings.ToLower(string(iter.Node().Data)))
	}
	return keys
}

// findTOMLValues also handles dotted keys and inline tables using the parser's
// nodes, rather than interpreting quotes or comments ourselves.
func findTOMLValues(n *unstable.Node, prefix, want []string) []unstable.Range {
	path := append(slices.Clone(prefix), tomlKeys(n)...)
	if slices.Equal(path, want) {
		return []unstable.Range{n.Value().Raw}
	}
	var ranges []unstable.Range
	if n.Value().Kind == unstable.InlineTable {
		iter := n.Value().Children()
		for iter.Next() {
			ranges = append(ranges, findTOMLValues(iter.Node(), path, want)...)
		}
	}
	return ranges
}

func configValueRanges(data []byte, table, key string) ([]unstable.Range, int, error) {
	p := unstable.Parser{}
	p.Reset(data)
	var scope []string
	var ranges []unstable.Range
	insert := -1
	for p.NextExpression() {
		n := p.Expression()
		if n.Kind == unstable.Table || n.Kind == unstable.ArrayTable {
			scope = tomlKeys(n)
			if slices.Equal(scope, []string{table}) {
				keys := n.Key()
				keys.Next()
				insert = tableLineEnd(data, int(keys.Node().Raw.Offset))
			}
			continue
		}
		if n.Kind == unstable.KeyValue {
			ranges = append(ranges, findTOMLValues(n, scope, []string{table, key})...)
		}
	}
	return ranges, insert, p.Error()
}

func setConfigValue(original []byte, table, key, literal string) ([]byte, error) {
	data := bytes.TrimPrefix(original, []byte("\xef\xbb\xbf"))
	ranges, insert, err := configValueRanges(data, table, key)
	if err != nil {
		return nil, err
	}
	if len(ranges) > 0 {
		// Distinct TOML keys can address the same config field through case
		// folding. Refresh all aliases so a later one cannot override the edit.
		for i := len(ranges) - 1; i >= 0; i-- {
			r := ranges[i]
			data = replaceBytes(data, int(r.Offset), int(r.Offset+r.Length), literal)
		}
		return restoreBOM(original, data), nil
	}
	if insert >= 0 {
		prefix := ""
		if insert > 0 && data[insert-1] != '\n' {
			prefix = lineEnding(data)
		}
		return restoreBOM(original, replaceBytes(data, insert, insert, prefix+key+" = "+literal+lineEnding(data))), nil
	}
	// A missing member in a root dotted/inline table needs special placement.
	return insertConfigValue(original, table, key, literal)
}

func replaceBytes(data []byte, start, end int, replacement string) []byte {
	out := append(bytes.Clone(data[:start]), replacement...)
	return append(out, data[end:]...)
}

func restoreBOM(original, data []byte) []byte {
	if bytes.HasPrefix(original, []byte("\xef\xbb\xbf")) {
		return append([]byte("\xef\xbb\xbf"), data...)
	}
	return data
}

func lineEnding(data []byte) string {
	if bytes.Contains(data, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

func tableLineEnd(data []byte, offset int) int {
	if n := bytes.IndexByte(data[offset:], '\n'); n >= 0 {
		return offset + n + 1
	}
	return len(data)
}

func insertConfigValue(original []byte, table, key, literal string) ([]byte, error) {
	data := bytes.TrimPrefix(original, []byte("\xef\xbb\xbf"))
	values, err := decodeTOML(data)
	if err != nil {
		return nil, err
	}
	exists := false
	for name := range values {
		exists = exists || strings.ToLower(name) == table
	}
	if !exists {
		return append(bytes.Clone(original), []byte(lineEnding(data)+"["+table+"]"+lineEnding(data)+key+" = "+literal+lineEnding(data))...), nil
	}
	// Root dotted tables accept another root dotted key. For inline tables,
	// insert at their opening brace (Raw on an inline table covers just '{').
	p := unstable.Parser{}
	p.Reset(data)
	for p.NextExpression() {
		n := p.Expression()
		if n.Kind == unstable.Table || n.Kind == unstable.ArrayTable {
			break
		}
		if n.Kind == unstable.KeyValue && slices.Equal(tomlKeys(n), []string{table}) && n.Value().Kind == unstable.InlineTable {
			start := int(n.Value().Raw.Offset) + 1
			separator := " "
			if n.Value().Child() != nil {
				separator = ", "
			}
			return restoreBOM(original, replaceBytes(data, start, start, key+" = "+literal+separator)), nil
		}
	}
	return restoreBOM(original, append([]byte(table+"."+key+" = "+literal+lineEnding(data)), data...)), nil
}
