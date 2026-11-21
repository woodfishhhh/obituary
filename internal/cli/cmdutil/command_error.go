package cmdutil

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

// Cobra does not expose typed positional-argument or required-flag errors.
// Only recognize its known forms; never rewrite the underlying error.
var argumentCount = regexp.MustCompile(`^(accepts (?:at most |between )?|requires at least )\d+( and \d+)? arg\(s\), (only )?received \d+$`)

func commandErrorMessage(err error) (id, detail string) {
	if e, ok := errors.AsType[*pflag.NotExistError](err); ok {
		return "cli.cobra.unknown_flag", e.GetSpecifiedName()
	}
	if e, ok := errors.AsType[*pflag.ValueRequiredError](err); ok {
		return "cli.cobra.missing_value", e.GetSpecifiedName()
	}
	if e, ok := errors.AsType[*pflag.InvalidValueError](err); ok {
		return "cli.cobra.invalid_value", e.GetFlag().Name + "=" + strconv.Quote(e.GetValue())
	}
	msg := err.Error()
	if rest, ok := strings.CutPrefix(msg, "unknown command "); ok {
		if q, e := strconv.QuotedPrefix(rest); e == nil && strings.HasPrefix(rest[len(q):], " for ") {
			return "cli.cobra.unknown_command", q
		}
	}
	if argumentCount.MatchString(msg) {
		return "cli.cobra.argument_count", msg
	}
	if rest, ok := strings.CutPrefix(msg, "required flag(s) "); ok && strings.HasSuffix(rest, " not set") {
		return "cli.cobra.required_flags", strings.TrimSuffix(rest, " not set")
	}
	if strings.HasPrefix(msg, "invalid argument ") && strings.Contains(msg, " for ") {
		return "cli.cobra.invalid_argument", msg
	}
	return "", ""
}

func machineErrors(cmd *cobra.Command, args []string) bool {
	for c := cmd; c != nil; c = c.Parent() {
		switch c.Name() {
		case "hook", "mcp", "rm", "rmdir", "remove-item", "git-clean", "__complete", "__completeNoDesc":
			return true
		}
	}
	// A parse error may precede --json, before Cobra sets the bound bool.
	for _, a := range args {
		if a == "--" {
			break
		}
		if a == "--json" || a == "--json=true" {
			return true
		}
	}
	return false
}

// CommandError renders framework diagnostics at the terminal boundary only.
func CommandError(cmd *cobra.Command, args []string, err error) string {
	if machineErrors(cmd, args) {
		return err.Error()
	}
	id, detail := commandErrorMessage(err)
	if id == "" {
		return err.Error()
	}
	l := Localizer()
	if l.Language() == i18n.English {
		return err.Error()
	}
	return l.Text(id, map[string]any{"Detail": detail})
}
