package integrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// ManualSteps reports a target obit cannot write safely. The plan stops and
// returns the exact steps the user must take; nothing was installed. Callers
// that only inspect status show the steps instead of treating them as a failure.
type ManualSteps struct{ Steps string }

func (m *ManualSteps) Error() string { return m.Steps }

func manualf(format string, args ...any) error {
	return &ManualSteps{Steps: fmt.Sprintf(format, args...)}
}

func boundedAgentScope(s Scope, root string) (Scope, error) {
	if !filepath.IsAbs(root) {
		return Scope{}, fmt.Errorf("agent installation requires an absolute scope root")
	}
	root, err := pathutil.Abs(root)
	if err != nil {
		return Scope{}, err
	}
	if err := s.checkInside(root, root); err != nil {
		return Scope{}, err
	}
	s.Global, s.Project = false, root
	return s, nil
}

// These planners use the host's instruction destination and Obituary's
// Markdown markers. Preserve bytes outside the owned block, including BOM
// and CRLF, to keep user instructions intact.
func planAgentRule(s Scope, path string, uninstall bool) (Change, error) {
	ch, err := readIntegrationChange(s, path)
	if err != nil {
		return ch, err
	}
	if !utf8.Valid(ch.orig) || bytes.ContainsRune(ch.orig, 0) {
		return ch, fmt.Errorf("%s: rules must be UTF-8 text", path)
	}
	content := string(ch.orig)
	start, end, err := markdownMarkedRange(content, blockBegin, blockEnd)
	if err != nil {
		return ch, fmt.Errorf("%s: %w", path, err)
	}
	eol := "\n"
	if strings.Contains(content, "\r\n") {
		eol = "\r\n"
	}
	block := strings.ReplaceAll(blockBegin+"\n"+strings.TrimRight(Awareness, "\n")+"\n"+blockEnd, "\n", eol)
	if start >= 0 {
		next := replaceMarkdownBlock(content, block, start, end, uninstall)
		if uninstall && strings.TrimSpace(strings.TrimPrefix(next, "\xef\xbb\xbf")) == "" {
			ch.Action, ch.delete = "remove", true
			return ch, nil
		}
		return setIntegrationData(ch, []byte(next)), nil
	}
	if uninstall {
		return ch, nil
	}
	if content != "" && content != "\xef\xbb\xbf" && !strings.HasSuffix(content, "\n") {
		content += eol
	}
	return setIntegrationData(ch, []byte(content+block+eol)), nil
}

func replaceMarkdownBlock(content, block string, start, end int, uninstall bool) string {
	if uninstall {
		block = ""
		if strings.HasPrefix(content[end:], "\r\n") {
			end += 2
		} else if strings.HasPrefix(content[end:], "\n") {
			end++
		}
	}
	return content[:start] + block + content[end:]
}

// Reuse the line-boundary checks for every Markdown block. A marker in an
// example is not ownership evidence; BOM and CRLF do not relax either edge.
func markdownMarkedRange(content, begin, end string) (int, int, error) {
	start, finish := strings.Index(content, begin), strings.Index(content, end)
	if strings.Count(content, begin) > 1 || strings.Count(content, end) > 1 || (start < 0) != (finish < 0) || finish < start {
		return -1, -1, fmt.Errorf("ambiguous Obituary markers; repair the block by hand")
	}
	if start < 0 {
		return -1, -1, nil
	}
	prefix, afterBegin, afterEnd := content[:start], content[start+len(begin):], content[finish+len(end):]
	if prefix != "" && prefix != "\xef\xbb\xbf" && !strings.HasSuffix(prefix, "\n") ||
		!strings.HasPrefix(afterBegin, "\n") && !strings.HasPrefix(afterBegin, "\r\n") ||
		finish > 0 && content[finish-1] != '\n' ||
		afterEnd != "" && !strings.HasPrefix(afterEnd, "\n") && !strings.HasPrefix(afterEnd, "\r\n") {
		return -1, -1, fmt.Errorf("markers for Obituary must be standalone lines")
	}
	return start, finish + len(end), nil
}

func pluginPayload(s Scope, path string, source []byte, executable string) ([]byte, error) {
	if !filepath.IsAbs(executable) || !isObitMCP(executable, []string{"mcp"}) {
		return nil, fmt.Errorf("plugin requires an absolute obit.exe path")
	}
	guard := s.GuardRM
	// A plain refresh retains an opted-in guard. Receipt validation in
	// PlanManagedAssets remains authoritative before any replacement.
	ch, err := readIntegrationChange(s, path)
	if err != nil {
		return nil, err
	}
	guard = guard || bytes.Contains(ch.orig, []byte(`"guard":true`))
	settings, err := marshal(map[string]any{"executable": executable, "guard": guard})
	if err != nil {
		return nil, err
	}
	// JSON text is also valid JS. Python templates decode a JSON string literal.
	if strings.HasSuffix(path, ".py") {
		settings, err = json.Marshal(string(settings))
		if err != nil {
			return nil, err
		}
		// Python stores escaped JSON inside its literal.
		if !guard && bytes.Contains(ch.orig, []byte(`\"guard\":true`)) {
			s.GuardRM = true
			return pluginPayload(s, path, source, executable)
		}
	}
	return bytes.ReplaceAll(source, []byte("__OBIT_SETTINGS__"), settings), nil
}

func unsupportedAgentSkill(agent string) func(Scope, bool) ([]Change, error) {
	return func(Scope, bool) ([]Change, error) {
		return nil, fmt.Errorf("%s: managed Skill discovery has not been verified; the installed rules describe Obituary; no Skill was installed", agent)
	}
}

func manualAgentMCP(agent, entryPoint, executable string, uninstall bool) ([]Change, error) {
	if uninstall {
		return nil, manualf("%s: open %s; remove only mcpServers.obituary after verifying command is obit.exe and args are [\"mcp\"]; automatic removal is unavailable", agent, entryPoint)
	}
	entry, err := (CommandArgsMCP{}).Encode(executable, nil)
	if err != nil {
		return nil, err
	}
	return nil, manualf("%s: open %s; merge {\"mcpServers\":{\"obituary\":%s}} while retaining other servers; no MCP registration was installed", agent, entryPoint, entry)
}
