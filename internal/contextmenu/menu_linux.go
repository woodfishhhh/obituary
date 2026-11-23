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
	env, err := userdirs.DesktopDirectoryEnv()
	if err != nil {
		return nil, err
	}
	desktopQuote := func(exe string) string {
		// The desktop entry reader unescapes before Exec tokenization. Percent is a field code even inside quotes.
		quoted := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "`", "\\`", "$", "\\$", "%", "%%").Replace(exe)
		quoted = "\"" + strings.ReplaceAll(quoted, "\\", "\\\\") + "\""
		return quoted
	}
	desktopExe := launcher(exe, env, desktopQuote)
	for _, a := range menuActions {
		label := a.label(lang)
		script := "#!/bin/sh\n# Obituary file action\n[ \"$#\" -gt 0 ] || exit 1\nexec " + a.command(launcher(exe, env, shellQuote), "\"$@\"") + "\n"
		files[filepath.Join(base, "nautilus/scripts/Obituary", label)] = menuFile(script, 0700)
		mime := "application/octet-stream;inode/directory;"
		if a.id == "policy" {
			mime = "inode/directory;"
		}
		desktop := "[Desktop Entry]\nType=Service\nMimeType=" + mime + "\nActions=obituary;\nX-KDE-Protocol=file\nX-KDE-Submenu=" + lang.Text(i18n.AppName) + "\n\n[Desktop Action obituary]\nName=" + label + "\nExec=" + a.command(desktopExe, "%F") + "\n"
		files[filepath.Join(base, "kio/servicemenus/obituary-"+a.id+".desktop")] = menuFile(desktop, 0700)
	}
	return files, nil
}
