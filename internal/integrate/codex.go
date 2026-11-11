package integrate

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// codexHandler carries a POSIX command (Codex in WSL, Linux, macOS) and a
// Windows one, so one hooks.json serves Codex wherever it runs.
type codexHandler struct {
	Type           string `json:"type"`
	Command        string `json:"command"`
	CommandWindows string `json:"commandWindows,omitempty"`
	Timeout        int    `json:"timeout,omitempty"`
}

func codexSpec(command, commandWindows string) hookSpec {
	var events []hookEvent
	for _, ev := range hook.CodexEvents {
		events = append(events, hookEvent{Event: ev.Event, Matcher: ev.Matcher, Handler: codexHandler{
			Type: "command", Command: command, CommandWindows: commandWindows, Timeout: ev.Timeout,
		}})
	}
	return hookSpec{Events: events, IsOurs: codexHookConfig().owns, PreserveNote: codexPreserveNote}
}

func codexHookConfig() HookConfig {
	return HookConfig{Agent: "codex", CommandKeys: []string{"command", "commandWindows"}}
}

func codexPreserveNote(raw json.RawMessage) string {
	var h codexHandler
	if json.Unmarshal(raw, &h) == nil && (isObitHook(h.Command, "codex") || isObitHook(h.CommandWindows, "codex")) {
		return "保留：含非讣告命令"
	}
	return ""
}

const (
	blockBegin = "<!-- obit:begin (managed by `obit init`; edits here are replaced) -->"
	blockEnd   = "<!-- obit:end -->"
)

// PlanCodex computes the changes for the Codex integration: hooks.json and
// an instruction block in AGENTS.md.
func PlanCodex(s Scope, command, commandWindows string, uninstall bool) ([]Change, error) {
	var hooksPath, agentsPath string
	if s.Global {
		dir, err := userdirs.CodexDir()
		if err != nil {
			return nil, err
		}
		hooksPath, agentsPath = filepath.Join(dir, "hooks.json"), filepath.Join(dir, "AGENTS.md")
	} else {
		hooksPath, agentsPath = filepath.Join(s.Project, ".codex", "hooks.json"), filepath.Join(s.Project, "AGENTS.md")
		for _, p := range []string{hooksPath, agentsPath} {
			if err := s.checkInside(s.Project, p, p+".bak"); err != nil {
				return nil, err
			}
		}
	}
	spec := codexSpec(command, commandWindows)
	noticeCommand, err := graveNoticeCommand(s, hooksPath, "codex", command)
	if err != nil {
		return nil, err
	}
	noticeWindows := commandWindows
	if commandWindows != "" && strings.HasSuffix(noticeCommand, " --notice-only") {
		noticeWindows += " --notice-only"
	}
	spec.Events = append(spec.Events, hookEvent{Event: "PreToolUse", Matcher: ".*", Handler: codexHandler{Type: "command", Command: noticeCommand, CommandWindows: noticeWindows}})
	h, err := planHooks(s, hooksPath, spec, uninstall)
	if err != nil {
		return nil, err
	}
	rules, err := planSharedAgentRules(s, agentsPath, "codex", uninstall)
	if err != nil {
		return nil, err
	}
	if !s.Global {
		h.project = s.Project
	}
	h.phase = 1
	if uninstall {
		h.phase = 41
	}
	return append([]Change{h}, rules...), nil
}

// planBlock writes text between Obituary's markers in a Markdown file,
// replacing an older block and leaving the rest of the file alone.
func planBlock(s Scope, path, text string, uninstall bool) (Change, error) {
	ch, err := readIntegrationChange(s, path)
	if err != nil {
		return ch, err
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
	block := strings.ReplaceAll(blockBegin+"\n"+strings.TrimRight(text, "\r\n")+"\n"+blockEnd, "\n", eol)
	if start >= 0 {
		if uninstall {
			block = ""
			if strings.HasPrefix(content[end:], "\r\n") {
				end += 2
			} else if strings.HasPrefix(content[end:], "\n") {
				end++
			}
		}
		next := content[:start] + block + content[end:]
		if uninstall && next == "" {
			ch.Action, ch.Detail, ch.delete = "remove", "only held Obituary's block", true
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

// checkInside refuses project paths that would reach
// outside the project through a link or junction: a cloned repository can
// carry a .codex or .claude link, or a planted .bak link, that redirects a
// write elsewhere.
func checkInside(root string, paths ...string) error {
	return (Scope{}).checkInside(root, paths...)
}

func (s Scope) checkInside(root string, paths ...string) error {
	for _, p := range paths {
		path, err := pathutil.Abs(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel != "." && !filepath.IsLocal(rel) {
			return fmt.Errorf("%s is outside the project %s", path, root)
		}
		if err := s.checkIntegrationAncestors(path); err != nil {
			return err
		}
	}
	return nil
}
