package integrate

import (
	"bytes"
	"fmt"
	"strings"
)

// planFile writes or removes a file Obituary owns outright (OBIT.md). It is
// only ever overwritten or deleted while it still carries ownedMarker: a file
// of the same name that someone else wrote is reported, never replaced.
func planFile(s Scope, path string, content []byte, uninstall bool) (Change, error) {
	ch, err := readIntegrationChange(s, path)
	if err != nil {
		return ch, err
	}
	data, exists := ch.orig, ch.existed
	// A quoted marker is not whole-file ownership. Legacy files have no
	// receipt, so only the complete shipped document proves its contents.
	ours := bytes.Equal(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")), bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n")))
	switch {
	case uninstall && exists && ours:
		ch.Action, ch.delete = "remove", true
	case uninstall:
		// absent, or not ours: leave it
	case !exists:
		ch.Action, ch.data = "create", content
	case !ours:
		return ch, fmt.Errorf("%s exists and was not written by Obituary; move it away, then run obit init again", path)
	case string(data) != string(content):
		ch.Action, ch.data, ch.Detail = "update", content, "refresh Obituary's instructions"
	}
	return ch, nil
}

// planReference adds or removes the single `@OBIT.md` line, preserving user text.
func planReference(s Scope, path, ref string, uninstall bool) (Change, error) {
	ch, err := readIntegrationChange(s, path)
	if err != nil {
		return ch, err
	}
	content := string(ch.orig)
	eol := "\n"
	if strings.Contains(content, "\r\n") {
		eol = "\r\n"
	}
	block := referenceBegin + eol + ref + eol + referenceEnd + eol
	start, end, err := markdownMarkedRange(content, referenceBegin, referenceEnd)
	if err != nil {
		return ch, err
	}
	if start >= 0 {
		return planExistingReference(ch, content, block, eol, start, end, uninstall), nil
	}
	if uninstall {
		return ch, nil
	}
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == ref {
			return ch, nil
		} // user-owned import
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += eol
	}
	ch = setIntegrationData(ch, []byte(content+block))
	return ch, nil
}

func planExistingReference(ch Change, content, block, eol string, start, end int, uninstall bool) Change {
	if content[start:end] != strings.TrimSuffix(block, eol) {
		ch.Action, ch.Detail = "preserve", "user-edited Obituary reference preserved"
		return ch
	}
	if !uninstall {
		return ch
	}
	if strings.HasPrefix(content[end:], eol) {
		end += len(eol)
	}
	ch = setIntegrationData(ch, []byte(content[:start]+content[end:]))
	if len(ch.data) == 0 {
		ch.Action, ch.delete = "remove", true
	}
	return ch
}

const referenceBegin = "<!-- obit:reference:begin -->"
const referenceEnd = "<!-- obit:reference:end -->"

func planClaudeReference(s Scope, ch Change, uninstall bool) ([]Change, error) {
	asset := ManagedAsset{Path: ch.Path + ".obituary-reference", ID: "claude-reference", Owner: "claude", Version: "1", Content: []byte("Obituary owns only its marked Claude import\n")}
	if !uninstall && ch.Action == "unchanged" && !strings.Contains(string(ch.orig), referenceBegin) {
		return []Change{ch}, nil // never claim a pre-existing bare user reference
	}
	return planSharedRegistration(s, asset, []Change{ch}, []string{"claude"}, uninstall)
}
