package hook

import "golang.org/x/sys/windows"

func systemDiscoveryRoots() []string {
	var roots []string
	for _, id := range []*windows.KNOWNFOLDERID{
		windows.FOLDERID_ProgramFiles, windows.FOLDERID_ProgramFilesX86, windows.FOLDERID_Windows,
	} {
		// Classification needs the configured location even when the hook
		// account cannot access the known folder itself.
		if path, err := windows.KnownFolderPath(id, windows.KF_FLAG_DONT_VERIFY); err == nil {
			roots = append(roots, path)
		}
	}
	return roots
}
