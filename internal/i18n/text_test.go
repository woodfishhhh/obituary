package i18n

import (
	"regexp"
	"slices"
	"testing"
)

func TestCatalogsComplete(t *testing.T) {
	verbs := regexp.MustCompile(`%[ds]`)
	for _, language := range []Language{Chinese, English} {
		catalog := catalogs[language]
		if len(catalog) != int(keyCount) {
			t.Fatalf("%s: %d keys, want %d", language, len(catalog), keyCount)
		}
		for key := Key(0); key < keyCount; key++ {
			if catalog[key] == "" {
				t.Errorf("%s: missing key %d", language, key)
			}
			if !slices.Equal(verbs.FindAllString(catalog[key], -1), verbs.FindAllString(english[key], -1)) {
				t.Errorf("%s: incompatible placeholders for %d", language, key)
			}
		}
	}
}

func TestMissingTextDoesNotFallBack(t *testing.T) {
	for _, language := range []Language{Chinese, English, "invalid"} {
		t.Run(string(language), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("missing text silently accepted")
				}
			}()
			_ = language.Text(keyCount)
		})
	}
}
