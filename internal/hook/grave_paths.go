package hook

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/woodfishhhh/obituary/internal/guardrm"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

// References are distinct from successful writes used by automatic registration.
// The schemas here are the Claude hook reference and Codex's native tool inputs
// (openai/codex rust-v0.167.0 core/src/tools/handlers and apply-patch/src/parser.rs).
func fileReferences(tool string, input map[string]any) ([]string, bool) {
	switch tool {
	case "Read", "read_file", "List", "LS", "list_directory", "list_dir", "Glob", "Grep":
		if p := firstString(input, "file_path", "path", "directory", "target_file", "target_directory"); p != "" {
			return []string{p}, false
		}
	case "Write", "Edit", "MultiEdit", "NotebookEdit", "write_file", "replace":
		if p := firstString(input, "file_path", "notebook_path", "path"); p != "" {
			return []string{p}, true
		}
	case "apply_patch", "functions.apply_patch":
		return PatchPaths(firstString(input, "command", "patch", "input")), true
	}
	return nil, false
}

// Codex's canonical Windows Bash envelope omits the actual shell. Only the
// intersection of the two literal read/list grammars is usable without config;
// quoting/escaping that yields different operands stays silent.
func shellReferences(ev Event) []string {
	if ev.Host == "codex" && ev.Shell == guardrm.Uncertain {
		bash := guardrm.ReferencedPaths(ev.Command, guardrm.Bash)
		ps := guardrm.ReferencedPaths(ev.Command, guardrm.PowerShell)
		if slices.Equal(bash, ps) {
			return bash
		}
		return nil
	}
	return guardrm.ReferencedPaths(ev.Command, ev.Shell)
}

func absentReferences(ev Event) []string {
	cwd, ok := NativePath(ev.Cwd)
	if !ok || !filepath.IsAbs(cwd) {
		return nil
	}
	refs := ev.References
	if ev.Command != "" && ev.SkipReason == "" {
		refs = append(refs, shellReferences(ev)...)
	}
	var absent []string
	for _, ref := range refs {
		p, ok := NativePath(ref)
		if !ok || p == "" || strings.HasPrefix(p, "~") || strings.ContainsAny(p, "*?\x00\r\n") {
			continue
		}
		if !filepath.IsAbs(p) {
			if strings.Contains(p, ":") || strings.HasPrefix(p, `\`) {
				continue
			}
			p = filepath.Join(cwd, p)
		}
		p, err := pathutil.Abs(p)
		if err != nil {
			continue
		}
		if _, err = os.Lstat(p); !errors.Is(err, os.ErrNotExist) {
			continue
		}
		// Expand existing ancestor short names; unavailable/ambiguous aliases skip.
		long, complete, err := (winfs.FS{}).LongPath(p)
		if err == nil && complete {
			absent = append(absent, long)
		}
	}
	return absent
}
