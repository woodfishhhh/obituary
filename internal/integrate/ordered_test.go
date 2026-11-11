package integrate

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func FuzzParseObject(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		o, err := parseObject(data)
		if err != nil {
			return
		}
		encoded, err := o.pretty()
		if err != nil {
			t.Fatal(err)
		}
		again, err := parseObject(encoded)
		if err != nil || !slices.Equal(o.keys, again.keys) {
			t.Fatalf("object key order changed: %v -> %v, %v", o.keys, again, err)
		}
		// UseNumber avoids losing large integer tokens in this independent oracle.
		decode := func(data []byte) any {
			t.Helper()
			var v any
			d := json.NewDecoder(bytes.NewReader(data))
			d.UseNumber()
			if err := d.Decode(&v); err != nil {
				t.Fatal(err)
			}
			return v
		}
		original := bytes.TrimSpace(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")))
		if len(original) == 0 {
			original = []byte("{}")
		}
		if !reflect.DeepEqual(decode(original), decode(encoded)) {
			t.Fatalf("object values changed: %q -> %q", data, encoded)
		}
		next, err := again.pretty()
		if err != nil || !bytes.Equal(encoded, next) {
			t.Fatalf("object formatting is not stable: %q -> %q, %v", encoded, next, err)
		}
		// Duplicate keys retain their first position but use their last value.
		// One duplicate per input keeps ordinary corpus runs cheap.
		if len(o.keys) > 0 {
			key := o.keys[0]
			keyJSON, _ := json.Marshal(key)
			duplicate := append(bytes.Clone(encoded[:bytes.LastIndexByte(encoded, '}')]), ',')
			duplicate = append(duplicate, keyJSON...)
			duplicate = append(duplicate, []byte(":null}")...)
			got, err := parseObject(duplicate)
			if err != nil || !slices.Equal(o.keys, got.keys) || string(got.values[key]) != "null" {
				t.Fatalf("duplicate key semantics changed for %q: %v", key, err)
			}
		}
	})
}
