package i18n

import (
	"encoding/json"
	"regexp"
	"slices"
	"testing"
)

func TestMessageFilesMatchEnglish(t *testing.T) {
	english := readMessageFile(t, "messages/active.en.json")
	if !slices.Equal(sortedKeys(english), sortedKeys(defaultMessages)) {
		t.Fatalf("English catalog keys do not match Go defaults: file=%v defaults=%v", sortedKeys(english), sortedKeys(defaultMessages))
	}
	placeholders := regexp.MustCompile("%[ds]|[{][{][^}]+[}][}]")
	for _, name := range []string{"messages/active.en.json", "messages/active.zh-CN.json"} {
		catalog := readMessageFile(t, name)
		if !slices.Equal(sortedKeys(catalog), sortedKeys(english)) {
			t.Fatalf("%s keys do not match English: got=%v want=%v", name, sortedKeys(catalog), sortedKeys(english))
		}
		for key, value := range english {
			if got, want := placeholders.FindAllString(catalog[key], -1), placeholders.FindAllString(value, -1); !slices.Equal(got, want) {
				t.Errorf("%s/%s placeholders: got=%v want=%v", name, key, got, want)
			}
		}
	}
}

func readMessageFile(t *testing.T, name string) map[string]string {
	t.Helper()
	data, err := messageFS.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	values := make(map[string]string, len(raw))
	for key, value := range raw {
		var text string
		if err := json.Unmarshal(value, &text); err != nil {
			var message map[string]string
			if err := json.Unmarshal(value, &message); err != nil {
				t.Fatalf("%s/%s: %v", name, key, err)
			}
			text = message["other"]
		}
		values[key] = text
	}
	return values
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
