package watch

import (
	"context"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func RunWithSource(ctx context.Context, svc *lifecycle.Service, add, remove func(string) error, close func() error, events <-chan fsnotify.Event, errs <-chan error) error {
	return run(ctx, svc, time.Hour, nil, watchSource{add, remove, close, events, errs})
}
