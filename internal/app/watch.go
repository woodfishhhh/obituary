package app

import (
	"context"
	"log"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/watch"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

// WatchFolders is the tray/resident integration point. Cancel ctx and wait for
// its return before closing the application; errors are reported without UI.
func WatchFolders(ctx context.Context, svc *core.Service, report func(error)) error {
	return watch.Run(ctx, svc, 5*time.Minute, report)
}

func startFolderWatch(svc *core.Service, logger *log.Logger) func() {
	ctx, cancel := context.WithCancel(svc.Store.Context())
	done := make(chan struct{})
	report := func(err error) { logger.Printf("watch: %v", err) }
	go func() {
		defer close(done)
		if err := WatchFolders(ctx, svc, report); err != nil {
			report(err)
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func allowDownloads(g core.Guard, paths []string) core.Guard {
	for _, path := range paths {
		g.WatchAllowed = append(g.WatchAllowed, path)
		if real, err := winfs.Canonical(path); err == nil {
			g.WatchAllowed = append(g.WatchAllowed, real)
		}
	}
	return g
}
