package integrate

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// Also bound user-level targets: the global configuration roots may be
// redirected, but none of these native installers writes through a junction.
func nativeSpecialScope(s Scope, path string) Scope {
	if s.Global {
		s.Global, s.Project = false, pathutil.VolumeRoot(path)
	}
	return s
}

func nativeSpecialOriginal(after, before Change) error {
	if after.existed != before.existed || !bytes.Equal(after.orig, before.orig) {
		return fmt.Errorf("%s changed during planning; retry", before.Path)
	}
	return nil
}

func nativeSpecialBlock(s Scope, path, text string, uninstall bool) (Change, error) {
	s = nativeSpecialScope(s, path)
	before, err := readIntegrationChange(s, path)
	if err != nil {
		return before, err
	}
	if !utf8.Valid(before.orig) || strings.Count(string(before.orig), blockBegin) != strings.Count(string(before.orig), blockEnd) || strings.Count(string(before.orig), blockBegin) > 1 {
		return before, fmt.Errorf("%s has invalid UTF-8 or ambiguous Obituary markers; left untouched", path)
	}
	ch, err := planBlock(s, path, text, uninstall)
	if err != nil {
		return ch, err
	}
	if err := nativeSpecialOriginal(ch, before); err != nil {
		return ch, err
	}
	ch.project = s.Project
	return ch, nil
}

// TOML planners read once to detect unmarked collisions, then use the shared
// AST editor. Its returned original must still match that collision check.
func nativeSpecialTOMLBase(s Scope, path, id string) (Change, map[string]any, error) {
	ch, err := readIntegrationChange(s, path)
	if err != nil {
		return ch, nil, err
	}
	data := bytes.TrimPrefix(ch.orig, []byte("\xef\xbb\xbf"))
	start, end, err := tomlMarkedRange(data, "# obit:"+id+":begin", "# obit:"+id+":end")
	if err != nil {
		return ch, nil, err
	}
	if start >= 0 {
		data = append(bytes.Clone(data[:start]), data[end:]...)
	}
	base, err := decodeTOML(data)
	return ch, base, err
}
