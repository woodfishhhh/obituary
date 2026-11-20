//go:build linux

package tray

import (
	"context"
	"log"
)

// StatusNotifierItem has no portable hide/show operation; the config switch
// remains available for future hosts, while the desktop entry stays usable.
func followVisibility(_ context.Context, _ *log.Logger) {}
