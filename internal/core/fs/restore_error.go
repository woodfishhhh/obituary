package fs

import "fmt"

// RestoreParentError describes a refusal before the buried payload was moved.
type RestoreParentError struct {
	Kind, Path, Current string
	Cause               error
}

func (e *RestoreParentError) Unwrap() error { return e.Cause }

func (e *RestoreParentError) Error() string {
	switch e.Kind {
	case "renamed":
		return fmt.Sprintf("original parent %s is currently at %s; the item remains in the graveyard", e.Path, e.Current)
	case "link":
		return fmt.Sprintf("the link at %s changed; its current location cannot be verified; the item remains in the graveyard", e.Path)
	case "permission":
		return fmt.Sprintf("permission denied inspecting %s; the item remains in the graveyard", e.Path)
	case "missing":
		return fmt.Sprintf("cannot resolve %s: no such file or directory (the link target may be missing); the item remains in the graveyard", e.Path)
	case "file":
		return fmt.Sprintf("original folder %s is now a file; the item is still in the graveyard; restore the original directory to its original path or recover manually", e.Path)
	case "file_windows":
		return fmt.Sprintf("original folder %s is now a file; the item is still in the graveyard, and after that file is moved or renamed, obit resurrect can restore it", e.Path)
	default:
		return fmt.Sprintf("original parent %s changed or cannot be verified; the item remains in the graveyard", e.Path)
	}
}
