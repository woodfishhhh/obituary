// Package explorer maintains owned Explorer display names without renaming files.
package explorer

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/woodfishhhh/obituary/internal/core"
)

const marker = "; obituary-labels:v1:"
const endMarker = "; obituary-labels:end\r\n"
const maxINIBytes = 1 << 20

type receipt struct {
	Directory core.Identity `json:"directory"`
	Original  []byte        `json:"original"`
	Existed   bool          `json:"existed"`
	FileAttrs uint32        `json:"file_attrs"`
	DirAttrs  uint32        `json:"dir_attrs"`
	Prefix    string        `json:"prefix"`
	Lines     string        `json:"lines"`
}

// go-ini/ini's file.go writeTo reserializes comments, spacing and blank lines.
// Windows WritePrivateProfileString also rewrites the source. Neither meets
// the byte-preserving merge contract: splice one owned block into raw text.
func decode(data []byte) (string, error) {
	if bytes.HasPrefix(data, []byte{0xff, 0xfe}) {
		if len(data)%2 != 0 {
			return "", errors.New("odd UTF-16 desktop.ini length")
		}
		units := make([]uint16, (len(data)-2)/2)
		for i := range units {
			units[i] = binary.LittleEndian.Uint16(data[2+i*2:])
		}
		text := string(utf16.Decode(units))
		if strings.IndexByte(text, 0) >= 0 || !slices.Equal(units, utf16.Encode([]rune(text))) {
			return "", errors.New("invalid UTF-16 desktop.ini")
		}
		return text, nil
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return "", errors.New("desktop.ini needs UTF-8 or BOM-marked UTF-16 LE; encoding not guessed")
	}
	return strings.TrimPrefix(string(data), "\ufeff"), nil
}

func encode(text string) []byte {
	units := utf16.Encode([]rune(text))
	data := make([]byte, 2+2*len(units))
	data[0], data[1] = 0xff, 0xfe
	for i, unit := range units {
		binary.LittleEndian.PutUint16(data[2+2*i:], unit)
	}
	return data
}

func unmerge(text string) (string, *receipt, error) {
	start := strings.Index(text, marker)
	if start < 0 {
		return text, nil, nil
	}
	if start > 0 && text[start-1] != '\n' {
		return "", nil, errors.New("misplaced label receipt")
	}
	line, _, ok := strings.Cut(text[start:], "\r\n")
	if !ok {
		return "", nil, errors.New("incomplete label receipt")
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(line, marker))
	if err != nil {
		return "", nil, err
	}
	var r receipt
	if err := json.Unmarshal(data, &r); err != nil {
		return "", nil, err
	}
	block := r.Prefix + line + "\r\n" + r.Lines + endMarker
	start -= len(r.Prefix)
	if start < 0 || !strings.HasPrefix(text[start:], block) || strings.Count(text, marker) != 1 {
		return "", nil, errors.New("owned Explorer labels changed; leaving desktop.ini untouched")
	}
	tail := text[start+len(block):]
	prefix := ""
	if strings.Contains(r.Prefix, "[LocalizedFileNames]") && sectionHasUserText(tail) {
		prefix = r.Prefix
	}
	return text[:start] + prefix + tail, &r, nil
}

// An inserted header also belongs to subsequent user keys/comments until the
// next section. Remove it only when that part of the section is still empty.
func sectionHasUserText(tail string) bool {
	for _, line := range strings.Split(tail, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			return false
		}
		if line != "" {
			return true
		}
	}
	return false
}

func section(text string) (int, map[string]bool) {
	offset, insert := 0, -1
	active := false
	keys := make(map[string]bool)
	// Windows also accepts lone CR. bufio.ScanLines only splits on LF and
	// strips terminators; retain both here so insertion offsets stay exact.
	for offset < len(text) {
		end := len(text)
		if n := strings.IndexAny(text[offset:], "\r\n"); n >= 0 {
			end = offset + n + 1
			if text[end-1] == '\r' && end < len(text) && text[end] == '\n' {
				end++
			}
		}
		line := text[offset:end]
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			active = strings.EqualFold(trimmed, "[LocalizedFileNames]")
			if active && insert < 0 {
				insert = offset + len(line)
			}
		} else if active && !strings.HasPrefix(trimmed, ";") && !strings.HasPrefix(trimmed, "#") {
			key, _, ok := strings.Cut(trimmed, "=")
			if ok {
				keys[strings.ToLower(strings.TrimSpace(key))] = true
			}
		}
		offset = end
	}
	return insert, keys
}

func merge(base string, labels map[string]string, r receipt) ([]byte, bool, error) {
	insert, keys := section(base)
	names := make([]string, 0, len(labels))
	for name := range labels {
		names = append(names, name)
	}
	slices.Sort(names)
	r.Lines = ""
	for _, name := range names {
		if !keys[strings.ToLower(name)] {
			r.Lines += name + "=" + labels[name] + "\r\n"
		}
	}
	if r.Lines == "" {
		return clean(base, r)
	}
	r.Prefix = ""
	if insert < 0 {
		insert, r.Prefix = len(base), "[LocalizedFileNames]\r\n"
	}
	if insert > 0 && base[insert-1] != '\n' {
		r.Prefix = "\r\n" + r.Prefix
	}
	data, err := json.Marshal(r)
	if err != nil {
		return nil, false, err
	}
	block := r.Prefix + marker + base64.StdEncoding.EncodeToString(data) + "\r\n" + r.Lines + endMarker
	data = encode(base[:insert] + block + base[insert:])
	if len(data) > maxINIBytes {
		return nil, false, errors.New("labelled desktop.ini exceeds 1 MiB")
	}
	return data, false, nil
}

func clean(base string, r receipt) ([]byte, bool, error) {
	original, err := decode(r.Original)
	if err != nil {
		return nil, false, fmt.Errorf("original desktop.ini: %w", err)
	}
	if base == original {
		return r.Original, !r.Existed, nil
	}
	return encode(base), false, nil
}
