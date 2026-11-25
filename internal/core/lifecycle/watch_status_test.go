package lifecycle_test

import (
	"runtime"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func TestWatchStatusUsesConfiguredWindowAndRetry(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	svc := &core.Service{WatchStableWindow: 30 * time.Second}
	it := core.Item{Source: "watch", ExpiresAt: now.Add(-time.Second), Watch: &core.WatchTiming{StableSince: now.Add(-10 * time.Second)}}
	status := svc.WatchStatus(it, now)
	wantWindow, wait := 30*time.Second, 20*time.Second
	if runtime.GOOS != "windows" {
		wantWindow, wait = 10*time.Minute, 10*time.Minute-time.Second
	}
	if status == nil || status.QuietFor != wait.String() || status.StableWindow != wantWindow.String() || !status.NextCheck.Equal(now) || !status.StableAt.Equal(now.Add(wait)) {
		t.Fatalf("quiet status: %+v", status)
	}
	it.NextAttempt = now.Add(time.Minute)
	if status = svc.WatchStatus(it, now); !status.NextCheck.Equal(it.NextAttempt) {
		t.Fatalf("retry status: %+v", status)
	}
	it.IsDir = true
	if svc.WatchStatus(it, now) != nil {
		t.Fatal("directory burial requires review")
	}
}
