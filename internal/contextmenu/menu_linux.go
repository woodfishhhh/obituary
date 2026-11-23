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
		register := func(id, filters string) {
			desktop := "[Desktop Entry]\nType=Service\n" + filters + "\nActions=obituary;\nX-KDE-Protocol=file\nX-KDE-Submenu=" + lang.Text(i18n.AppName) + "\n\n[Desktop Action obituary]\nName=" + label + "\nExec=" + a.command(desktopExe, "%F") + "\n"
			files[filepath.Join(base, "kio/servicemenus/obituary-"+id+".desktop")] = menuFile(desktop, 0700)
		}
		// Dolphin passes the current directory as one URL for a background click, exactly like a selected folder.
		if a.id == "policy" {
			register(a.id, "MimeType=inode/directory;\nX-KDE-MinNumberOfUrls=2")
		} else {
			register(a.id, "MimeType=all/allfiles;\nExcludeServiceTypes=inode/directory\nX-KDE-MinNumberOfUrls=1\nX-KDE-MaxNumberOfUrls=1")
			register(a.id+"-multi", "MimeType=all/all;\nX-KDE-MinNumberOfUrls=2")
		}
	}
	return files, nil
}
