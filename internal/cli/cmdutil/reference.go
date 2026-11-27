package cmdutil

import (
	"errors"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

func ReferenceText(l *i18n.Localizer, err error, raw string) string {
	if e, ok := errors.AsType[*corefs.RestoreParentError](err); ok {
		return l.Text("cli.restore_parent."+e.Kind, map[string]any{"Path": e.Path, "Current": e.Current})
	}
	if l.Language() == i18n.English {
		return raw
	}
	e, ok := errors.AsType[*lifecycle.ReferenceError](err)
	if !ok {
		return raw
	}
	return l.Text("cli.reference."+e.Kind, map[string]any{"Ref": e.Ref, "ID": e.BurialID})
}
