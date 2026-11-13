//go:build windows

package integrate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
	"github.com/woodfishhhh/obituary/internal/config"
)

func TestGraveyardConfigProtectsTemporarySource(t *testing.T) {
	for _, exists := range []bool{false, true} {
		t.Run(fmt.Sprint(exists), func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "config.toml")
			original := "# keep comment\n[graveyard]\nmode = 'graveyard'\nretention_days = 7\n"
			if exists {
				writeIntegrationFile(t, path, original)
			}
			c, err := PlanGraveyardConfig(path, "recycle", 30)
			if err != nil {
				t.Fatal(err)
			}
			protectIntegrationTemp(t, root, "in-place")
			if err := Apply([]Change{c}); err != nil {
				t.Fatal(err)
			}
			if read(t, path) != string(c.data) {
				t.Fatal("graveyard settings published unapproved temporary bytes")
			}
			if exists && read(t, path+".bak") != original {
				t.Fatal("graveyard settings lost the verified original")
			}
			assertIntegrationTempRemoved(t, root)
		})
	}
}

func FuzzGraveyardConfig(f *testing.F) {
	for _, seed := range []string{
		"", "\xef\xbb\xbf", "# no final newline", "[graveyard]",
		"[graveyard]\nmode = 'recycle' # keep\nretention_days = 0x0e\n",
		"\xef\xbb\xbf# 注释\r\n[graveyard]\r\nmode = \"graveyard\"\r\nretention_days = +7 # keep\r\n[other]\r\nx = [1, 2]\r\n",
		"graveyard = {} # empty inline\n",
		"graveyard = { mode = 'recycle', retention_days = 7, user = { x = 'keep' } }\n",
		"GRAVEYARD = {Mode='recycle', retention_dAYs=1, RETENTION_DAYS=7}\n",
		"[GRAVEYARD]\nMODE='recycle'\n[graveyard]\nmode='graveyard'\nretention_DAYS=14\n",
		"[GRAVEYARD.user]\nx='keep'\n",
		"graveyard.mode = '''graveyard'''\ngraveyard.retention_days = 7\n",
		"[graveyard.user]\nx = 'keep'\n",
		"[other]\nmode = 'keep'\nretention_days = 99\n",
		"[[other]]\nx = nan\n[[other]]\nx = +inf\n",
		"[graveyard]\nmode = '''\ngraveyard'''\nretention_days = 7\n",
		"[graveyard]\nmode = 'graveyard'\nmode = 'recycle'\n",
		"graveyard = {", "graveyard.mode = 1\n", "\x00", "x='unterminated",
	} {
		f.Add([]byte(seed), uint16(7), false)
	}
	path := filepath.Join(f.TempDir(), "config.toml")
	f.Fuzz(func(t *testing.T, original []byte, days uint16, recycle bool) {
		if err := os.WriteFile(path, original, 0o600); err != nil {
			t.Fatal(err)
		}
		mode := "graveyard"
		if recycle {
			mode = "recycle"
		}
		retention := int(days%3650) + 1
		ch, err := PlanGraveyardConfig(path, mode, retention)
		unchanged, readErr := os.ReadFile(path)
		if readErr != nil || !bytes.Equal(unchanged, original) {
			t.Fatalf("planning modified input: %v", readErr)
		}
		_, inputErr := config.Parse(original, path)
		if inputErr != nil {
			if err == nil || ch.Action != "unchanged" || ch.data != nil {
				t.Fatalf("invalid configuration produced a writable plan: %v", inputErr)
			}
			return
		}
		if err != nil {
			t.Fatalf("valid configuration refused: %v", err)
		}
		out := ch.data
		if ch.Action == "unchanged" {
			out = original
		}
		cfg, err := config.Parse(out, path)
		if err != nil || cfg.Graveyard.Mode != mode || cfg.Graveyard.RetentionDays != retention {
			t.Fatalf("invalid planned configuration: %+v %v", cfg.Graveyard, err)
		}
		assertGraveyardSemantics(t, original, out)
		current := bytes.Clone(original)
		for _, edit := range [][2]string{{"mode", strconv.Quote(mode)}, {"retention_days", strconv.Itoa(retention)}} {
			next, err := setConfigValue(current, "graveyard", edit[0], edit[1])
			if err != nil {
				t.Fatal(err)
			}
			assertGraveyardBytes(t, current, next, edit[0], edit[1])
			current = next
		}
		if !bytes.Equal(out, current) {
			t.Fatal("public plan differs from checked edits")
		}
	})
}

func assertGraveyardSemantics(t *testing.T, before, after []byte) {
	t.Helper()
	var want, got map[string]any
	bom := []byte("\xef\xbb\xbf")
	if err := toml.Unmarshal(bytes.TrimPrefix(before, bom), &want); err != nil {
		t.Fatal(err)
	}
	if err := toml.Unmarshal(bytes.TrimPrefix(after, bom), &got); err != nil {
		t.Fatal(err)
	}
	for name, value := range got {
		if strings.ToLower(name) != "graveyard" {
			continue
		}
		grave := value.(map[string]any)
		pruneGraveyardFields(grave)
		if _, existed := want[name]; !existed && len(grave) == 0 {
			delete(got, name)
		}
	}
	for name, value := range want {
		if strings.ToLower(name) == "graveyard" {
			pruneGraveyardFields(value.(map[string]any))
		}
	}
	// Marshal canonical maps instead of DeepEqual: valid unknown values can
	// contain NaN, which does not compare equal to itself.
	left, err := toml.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	right, err := toml.Marshal(got)
	if err != nil || !bytes.Equal(left, right) {
		t.Fatalf("unrelated TOML values changed: %s -> %s (%v)", left, right, err)
	}
}

func pruneGraveyardFields(grave map[string]any) {
	for key := range grave {
		folded := strings.ToLower(key)
		if folded == "mode" || folded == "retention_days" {
			delete(grave, key)
		}
	}
}

func assertGraveyardBytes(t *testing.T, before, after []byte, key, literal string) {
	t.Helper()
	bom := []byte("\xef\xbb\xbf")
	data := bytes.TrimPrefix(before, bom)
	p := unstable.Parser{}
	p.Reset(data)
	var table []string
	var values []unstable.Range
	for p.NextExpression() {
		n := p.Expression()
		switch n.Kind {
		case unstable.Table, unstable.ArrayTable:
			table = fuzzTOMLKeys(n)
		case unstable.KeyValue:
			values = append(values, fuzzTOMLLiterals(n, table, key)...)
		}
	}
	if len(values) > 0 {
		want := string(before)
		for i := len(values) - 1; i >= 0; i-- {
			start := len(before) - len(data) + int(values[i].Offset)
			end := start + int(values[i].Length)
			want = want[:start] + literal + want[end:]
		}
		if string(after) != want {
			t.Fatal("bytes outside target literal changed")
		}
		return
	}
	// Adding an absent key may insert bytes, but may not replace, reorder or
	// discard any original byte (including the BOM and newline spelling).
	start := 0
	for start < len(before) && start < len(after) && before[start] == after[start] {
		start++
	}
	extra := len(after) - len(before)
	if extra < 0 || start+extra > len(after) || !bytes.Equal(before[start:], after[start+extra:]) {
		t.Fatal("adding a missing key rewrote original bytes")
	}
}

func fuzzTOMLKeys(n *unstable.Node) []string {
	var keys []string
	it := n.Key()
	for it.Next() {
		keys = append(keys, string(it.Node().Data))
	}
	return keys
}

func fuzzTOMLLiterals(n *unstable.Node, table []string, key string) []unstable.Range {
	path := append(slices.Clone(table), fuzzTOMLKeys(n)...)
	if len(path) == 2 && strings.ToLower(path[0]) == "graveyard" && strings.ToLower(path[1]) == key {
		return []unstable.Range{n.Value().Raw}
	}
	var ranges []unstable.Range
	if n.Value().Kind == unstable.InlineTable {
		it := n.Value().Children()
		for it.Next() {
			ranges = append(ranges, fuzzTOMLLiterals(it.Node(), path, key)...)
		}
	}
	return ranges
}

func TestGraveyardConfigPreservesLiteralFormatting(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		path := filepath.Join(t.TempDir(), "config.toml")
		original := "\xef\xbb\xbf# keep\n['graveyard'] # table\n  'mode'\t= '''recycle''' # mode\nretention_days=0x0e # days\n[agent_shells]\nmode='bash'\nretention_days='bash'\n"
		original = strings.ReplaceAll(original, "\n", newline)
		if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
			t.Fatal(err)
		}
		ch, err := PlanGraveyardConfig(path, "graveyard", 30)
		want := strings.Replace(original, "'''recycle'''", `"graveyard"`, 1)
		want = strings.Replace(want, "0x0e", "30", 1)
		if err != nil || string(ch.data) != want {
			t.Fatalf("literal edit: %q %v", ch.data, err)
		}
	}
}

func TestGraveyardConfigPreservesCaseAliases(t *testing.T) {
	for _, tc := range []struct{ before, want string }{
		{
			"Graveyard = { mode='graveyard', Mode='recycle', RETENTION_days=7, retention_DAYS=14 } # keep\n",
			"Graveyard = { mode=\"recycle\", Mode=\"recycle\", RETENTION_days=30, retention_DAYS=30 } # keep\n",
		},
		{
			"[GRAVEYARD]\nMODE='graveyard'\nretention_DAYS=7\n[graveyard]\nmode='recycle'\nRETENTION_days=14\n[agent_shells]\nMode='bash'\n",
			"[GRAVEYARD]\nMODE=\"recycle\"\nretention_DAYS=30\n[graveyard]\nmode=\"recycle\"\nRETENTION_days=30\n[agent_shells]\nMode='bash'\n",
		},
	} {
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(path, []byte(tc.before), 0o600); err != nil {
			t.Fatal(err)
		}
		ch, err := PlanGraveyardConfig(path, "recycle", 30)
		if err != nil || string(ch.data) != tc.want {
			t.Fatalf("case aliases: %q %v", ch.data, err)
		}
	}
}
