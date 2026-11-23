package contextmenu

import (
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"path/filepath"
	"strings"
)

func nativeMenus(exe string, lang i18n.Language) (map[string]Tree, error) {
	base, err := userdirs.TrashData()
	if err != nil {
		return nil, err
	}
	files := map[string]Tree{}
	// The desktop entry reader unescapes before Exec tokenization. Percent is a field code even inside quotes.
	quoted := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "\u0060", "\\u0060", "\u0024", "\\u0024", "%", "%%").Replace(exe)
	quoted = "\"" + strings.ReplaceAll(quoted, "\\", "\\\\") + "\""
	for _, a := range menuActions {
		label := a.label(lang)
		script := "#!/bin/sh\n# Obituary file action\n[ \"\u0024#\" -gt 0 ] || exit 1\nexec " + a.command(shellQuote(exe), "\"\u0024@\"") + "\n"
		files[filepath.Join(base, "nautilus/scripts/Obituary", label)] = menuFile(script, 0700)
		mime := "application/octet-stream;inode/directory;"
		if a.id == "policy" {
			mime = "inode/directory;"
		}
		desktop := "[Desktop Entry]\nType=Service\nMimeType=" + mime + "\nActions=obituary;\nX-KDE-Protocol=file\nX-KDE-Submenu=" + lang.Text(i18n.AppName) + "\n\n[Desktop Action obituary]\nName=" + label + "\nExec=" + a.command(quoted, "%F") + "\n"
		files[filepath.Join(base, "kio/servicemenus/obituary-"+a.id+".desktop")] = menuFile(desktop, 0700)
	}
	return files, nil
}
