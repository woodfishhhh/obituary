package app

import (
	"context"
	"os"
	"time"

	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/update"
)

const updateStartupDelay = 5 * time.Second

// startUpdateChecks keeps network waits outside the resident's ledger loop.
// Configuration is reread before each attempt so disabling applies while running.
func startUpdateChecks(ctx context.Context, home string) func() {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if os.Getenv("OBIT_TEST_NO_UPDATE") == "1" {
			return
		}
		timer := time.NewTimer(updateStartupDelay)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				checkUpdates(ctx, home)
				timer.Reset(time.Hour)
			}
		}
	}()
	return func() { cancel(); <-done }
}

// CheckUpdatesOnce lets the scheduled one-shot runner share the daily cache.
// A fresh cache or a disabled check finishes without the startup delay.
func CheckUpdatesOnce(home string) {
	if os.Getenv("OBIT_TEST_NO_UPDATE") == "1" {
		return
	}
	cfg, err := config.Load()
	if err != nil || !cfg.Update.Check {
		return
	}
	state, _ := update.Read(home)
	if !state.CheckedAt.IsZero() && time.Since(state.CheckedAt) < update.Interval {
		return
	}
	time.Sleep(updateStartupDelay)
	checkUpdates(context.Background(), home)
}

func checkUpdates(ctx context.Context, home string) {
	cfg, err := config.Load()
	if err == nil {
		_, _ = (update.Checker{Home: home}).Check(ctx, cfg.Update.Check, false)
	}
}
