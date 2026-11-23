package winfs

import (
	"fmt"
	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

func (FS) Recycle(string, corefs.Identity) error {
	return fmt.Errorf("this platform supports only the graveyard; set mode = \"graveyard\" to bury files: %w", corefs.ErrUnsupported)
}
