package i18n

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

func TestMissingTranslationRendersEnglishDefault(t *testing.T) {
	bundle := goi18n.NewBundle(language.English)
	if err := bundle.AddMessages(language.SimplifiedChinese, &goi18n.Message{ID: "other", Other: "other"}); err != nil {
		t.Fatal(err)
	}
	l := &Localizer{language: Chinese, localizer: goi18n.NewLocalizer(bundle, string(Chinese))}
	if got := l.Text("cli.warning", map[string]any{"Text": "sample"}); got != "warning: sample" {
		t.Fatalf("got %q", got)
	}
}

func TestMessageFilesMatchEnglish(t *testing.T) {
	english := readMessageFile(t, "messages/active.en.json")
	if !slices.Equal(sortedKeys(english), sortedKeys(defaultMessages)) {
		t.Fatalf("English catalog keys do not match Go defaults: file=%v defaults=%v", sortedKeys(english), sortedKeys(defaultMessages))
	}
	entries, err := messageFS.ReadDir("messages")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "active.en.json" || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := "messages/" + entry.Name()
		if err := validateMessageCatalog(english, readMessageFile(t, name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestEveryCLILanguageHasNonemptyMessages(t *testing.T) {
	for _, tag := range supportedTags {
		lang := tag.String()
		catalog := readMessageFile(t, "messages/active."+lang+".json")
		for id := range defaultMessages {
			if strings.TrimSpace(catalog[id]) == "" {
				t.Errorf("%s: missing or blank message %s", lang, id)
			}
		}
	}
}

func TestMessageCatalogValidationRejectsMissingDirectory(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "missing", "active.zh-CN.json"))
	if err != nil {
		t.Fatal(err)
	}
	catalog := map[string]string{}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	err = validateMessageCatalog(readMessageFile(t, "messages/active.en.json"), catalog)
	if err == nil {
		t.Fatalf("intentional missing catalog was accepted: %v", err)
	}
	t.Logf("intentional missing catalog rejected: %v", err)
}

func validateMessageCatalog(english, catalog map[string]string) error {
	problems := []string{}
	for _, key := range sortedKeys(english) {
		if _, ok := catalog[key]; !ok {
			problems = append(problems, "missing key "+key)
		}
	}
	for _, key := range sortedKeys(catalog) {
		if _, ok := english[key]; !ok {
			problems = append(problems, "extra key "+key)
		}
	}
	placeholders := regexp.MustCompile(`%[-+#0-9.]*[dsqtwv]|\{\{[^}]+\}\}`)
	for key, value := range english {
		translated, ok := catalog[key]
		if !ok {
			continue
		}
		if got, want := placeholders.FindAllString(translated, -1), placeholders.FindAllString(value, -1); !slices.Equal(got, want) {
			problems = append(problems, fmt.Sprintf("placeholder mismatch %s: got=%v want=%v", key, got, want))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return errors.New(strings.Join(problems, "; "))
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
