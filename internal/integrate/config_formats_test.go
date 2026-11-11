package integrate

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigEditsExpectedAndNoop(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	for _, format := range []ConfigFormat{ConfigJSON, ConfigJSONC, ConfigYAML} {
		t.Run(string(format), func(t *testing.T) {
			s := Scope{Project: t.TempDir()}
			path := filepath.Join(s.Project, "config")
			input := "\xef\xbb\xbf" + `{"z":9007199254740993,"keep":"<&>","settings":{"enabled":true}}`
			writeIntegrationFile(t, path, input)
			edit := ConfigEdit{Path: []string{"settings", "enabled"}, Value: json.RawMessage(`true`), Expected: json.RawMessage(`true`), CheckExpected: true}
			ch, err := PlanConfigEdits(s, path, format, []ConfigEdit{edit})
			if err != nil || ch.Action != "unchanged" {
				t.Fatalf("no-op: %+v %v", ch, err)
			}
			edit.Expected = json.RawMessage(`false`)
			if _, err := PlanConfigEdits(s, path, format, []ConfigEdit{edit}); err == nil {
				t.Fatal("expected-value mismatch accepted")
			}
			edit.Expected, edit.Value = json.RawMessage(`true`), json.RawMessage(`false`)
			ch, err = PlanConfigEdits(s, path, format, []ConfigEdit{edit})
			applyIntegration(t, []Change{ch}, err)
			if read(t, testBackupPath(t, path)) != input || !strings.HasPrefix(read(t, path), "\xef\xbb\xbf") {
				t.Fatal("BOM or backup changed")
			}
			got, err := configJSON([]byte(read(t, path)), format)
			if err != nil || !sameConfigJSON(got, []byte(`{"z":9007199254740993,"keep":"<&>","settings":{"enabled":false}}`)) {
				t.Fatalf("wrong semantics: %s %v", got, err)
			}
		})
	}
	if _, err := editConfig([]byte(`{}`), ConfigJSON, []ConfigEdit{{Path: []string{"missing", "child"}, Remove: true, CheckExpected: true, Expected: json.RawMessage(`1`)}}); err == nil {
		t.Fatal("expected child silently ignored under absent parent")
	}
}

func TestConfigEditsRejectMalformedAndJSON5(t *testing.T) {
	t.Parallel()
	for _, edit := range []ConfigEdit{{Value: json.RawMessage(`1`)}, {Path: []string{"a"}, Value: json.RawMessage(`broken`)}, {Path: []string{"a", "b"}, Value: json.RawMessage(`1`)}} {
		if _, err := editConfig([]byte(`{"a":null}`), ConfigJSON, []ConfigEdit{edit}); err == nil {
			t.Fatal("invalid edit accepted", edit)
		}
	}
	edits := []ConfigEdit{{Path: []string{"mcp", "servers", "obituary"}, Value: json.RawMessage(`{"command":"obit","args":["mcp"]}`)}}
	if _, err := editConfig([]byte(`{ // comment
 unquoted: 'value',
}`), ConfigJSON5, edits); err == nil || !strings.Contains(err.Error(), `["mcp","servers","obituary"]`) || !strings.Contains(err.Error(), `"command":"obit"`) {
		t.Fatalf("missing manual steps: %v", err)
	}
}
