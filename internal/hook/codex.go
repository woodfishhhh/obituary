package hook

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
)

// codexInput is the part of a Codex hook payload Obituary reads. Codex edits
// files with apply_patch, whose patch text arrives in tool_input.command.
type codexInput struct {
	SessionID string         `json:"session_id"`
	Cwd       string         `json:"cwd"`
	Event     string         `json:"hook_event_name"`
	Source    string         `json:"source"`
	StopHook  bool           `json:"stop_hook_active"`
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
}

// CodexEvents are the events `obit init --agent codex` registers. Codex
// gives SessionEnd hooks 1 second unless they ask for up to 3.
var CodexEvents = []struct {
	Event, Matcher string
	Timeout        int
}{
	{"SessionStart", "", 0},
	{"PostToolUse", "apply_patch|Edit|Write", 0},
	{"Stop", "", 0},
	{"SessionEnd", "", 3},
}

// Codex handles one Codex hook event; like Claude it fails open on errors.
func Codex(e *auto.Engine, in io.Reader, out io.Writer, runner *Runner) {
	Run("codex", e, in, out, runner)
}

type codexAdapter struct{ claudeAdapter }

func (codexAdapter) Decode(raw []byte, _ Invocation) (Event, error) {
	var input codexInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return Event{}, err
	}
	cwd := input.Cwd
	if input.Event != "PreToolUse" {
		var ok bool
		cwd, ok = NativePath(cwd)
		if !ok {
			if input.Event == "PostToolUse" && strings.Contains(firstString(input.ToolInput, "command", "patch", "input"), "*** Begin Patch") {
				// A plain error is logged to hook.log; the fallback stays silent.
				return Event{}, errors.New("codex PostToolUse: apply_patch skipped: cwd cannot be verified as local")
			}
			return Event{}, ErrSkip // a path this build cannot reach (for example inside WSL's own file system)
		}
	}
	ev := Event{Name: input.Event, Tool: input.ToolName, Session: core.NormalizeSession(input.SessionID), Cwd: cwd, Source: input.Source}
	// Codex Stop has no additionalContext; block + reason continues the turn.
	ev.CanContinue, ev.StopActive = input.Event == "Stop", input.StopHook
	ev.References, ev.Writing = fileReferences(input.ToolName, input.ToolInput)
	if input.Event == "PreToolUse" {
		ev.Cwd = codexWorkdir(input.Cwd, firstString(input.ToolInput, "workdir", "cwd"))
		ev.Command, ev.Shell, ev.SkipReason = codexCommand(input.ToolName, input.ToolInput, ev.Cwd)
	}
	if input.Event == "PostToolUse" {
		return codexWriteEvent(ev, input)
	}
	return ev, nil
}

func codexWriteEvent(ev Event, input codexInput) (Event, error) {
	if path := firstString(input.ToolInput, "file_path"); path != "" {
		ev.Paths = append(ev.Paths, path)
	}
	if cmd := firstString(input.ToolInput, "command", "patch", "input"); strings.Contains(cmd, "*** Begin Patch") {
		paths, err := localPatchPaths(cmd)
		if err != nil {
			return Event{}, fmt.Errorf("codex PostToolUse: %w", err)
		}
		ev.Paths = append(ev.Paths, paths...)
	}
	var native []string
	for _, p := range ev.Paths {
		if np, ok := NativePath(p); ok {
			native = append(native, np)
		}
	}
	ev.Paths = native
	return ev, nil
}

func init() {
	Register(Agent{Name: "codex", Adapter: codexAdapter{}, Capabilities: Capabilities{
		DeleteGuard:      Support{Level: Supported, Detail: "Bash/PowerShell envelopes or an owner-declared Windows shell; opt-in with --guard-rm"},
		WriteObservation: Support{Level: Supported}, SessionStart: Support{Level: Supported},
		Stop: Support{Level: Supported}, SessionEnd: Support{Level: Supported},
		Context: Support{Level: Supported, Detail: "SessionStart and PostToolUse; Stop asks once per session about files the agent created and left unsorted; SessionEnd stays silent"},
		MCP:     Support{Level: Supported}, Skill: Support{Level: Supported},
	}})
}

// Resolve tool-relative paths in the session's namespace before inferring the
// shell or WSL scope. filepath.Join alone would apply Windows rules to WSL.
func codexWorkdir(cwd, workdir string) string {
	if workdir == "" {
		return cwd
	}
	if strings.HasPrefix(cwd, "/") {
		if path.IsAbs(workdir) {
			return path.Clean(workdir)
		}
		return path.Join(cwd, workdir)
	}
	if windowsCwd(cwd) && !filepath.IsAbs(workdir) && filepath.VolumeName(workdir) == "" {
		if strings.HasPrefix(workdir, `\`) || strings.HasPrefix(workdir, "/") {
			return filepath.Clean(filepath.VolumeName(cwd) + workdir)
		}
		return filepath.Join(cwd, workdir)
	}
	return workdir
}

// WSLExecutable is shared by installed hooks and copyable delete suggestions.
// It returns an absolute, Bash-quoted Windows executable path for WSL interop.
func WSLExecutable(self string) string {
	vol := filepath.VolumeName(self)
	if len(vol) != 2 || vol[1] != ':' {
		return ""
	}
	path := "/mnt/" + strings.ToLower(vol[:1]) + filepath.ToSlash(self[2:])
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

// PatchPaths lists the local files an apply_patch patch adds, updates or moves to.
// Deleted files and patches with unverified execution environments are left out.
func PatchPaths(patch string) []string {
	paths, _ := localPatchPaths(patch)
	return paths
}

func localPatchPaths(patch string) ([]string, error) {
	var paths []string
	preamble := true
	scanner := bufio.NewScanner(strings.NewReader(patch))
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		trimmed := strings.TrimSpace(line)
		// openai/codex rust-v0.167.0: codex-rs/apply-patch/src/parser.rs
		// (test_parse_patch_environment_id_preamble) and streaming_parser.rs
		// trim preamble markers and accept ':' without a following space.
		// Environment IDs require a turn's environment map, absent from hooks.
		if strings.HasPrefix(line, "*** Environment ID:") || preamble && strings.HasPrefix(trimmed, "*** Environment ID:") {
			return nil, fmt.Errorf("apply_patch skipped: Environment ID cannot be verified as local")
		}
		if line == "*** Begin Patch" {
			preamble = true
		} else if strings.HasPrefix(trimmed, "*** Add File:") || strings.HasPrefix(trimmed, "*** Update File:") || strings.HasPrefix(trimmed, "*** Delete File:") {
			preamble = false
		}
		// File headers start in column 0; hunk lines start with ' ', '+' or '-'.
		for _, prefix := range []string{"*** Add File:", "*** Update File:", "*** Move to:"} {
			if strings.HasPrefix(line, prefix) {
				if p := strings.TrimSpace(strings.TrimPrefix(line, prefix)); p != "" {
					paths = append(paths, p)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("apply_patch skipped: execution environment could not be verified: %w", err)
	}
	return paths, nil
}

// NativePath maps a path from the agent to one this build can use. On
// Windows, WSL's /mnt/<drive>/... becomes <DRIVE>:\...; relative paths pass
// through; other POSIX absolute paths are unreachable (ok is false).
func NativePath(p string) (string, bool) {
	if runtime.GOOS != "windows" && strings.Contains(p, "://") {
		return "", false
	}
	if runtime.GOOS != "windows" || !strings.HasPrefix(p, "/") {
		return p, p != ""
	}
	p = path.Clean(p)
	if len(p) >= 6 && strings.HasPrefix(p, "/mnt/") && isLetter(p[5]) && (len(p) == 6 || p[6] == '/') {
		rest := ""
		if len(p) > 7 {
			rest = p[7:]
		}
		return strings.ToUpper(p[5:6]) + `:\` + filepath.FromSlash(rest), true
	}
	return "", false
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
