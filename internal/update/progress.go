package update

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

// What the update helper is doing, for the window that started it. The window
// stays open through the download and closes once Phase is Ready.
const (
	PhaseStarting    = "starting"
	PhaseDownloading = "downloading"
	PhaseVerifying   = "verifying"
	PhaseReady       = "ready" // nothing replaced yet; the window may close now
	PhaseInstalling  = "installing"
	PhaseDone        = "done"
	PhaseFailed      = "failed"
)

type Progress struct {
	Phase string `json:"phase"`
	Done  int64  `json:"done,omitempty"`
	Total int64  `json:"total,omitempty"`
	Error string `json:"error,omitempty"`
}

// ReadProgress returns the zero value when no update has run.
func ReadProgress(home string) (Progress, error) {
	var p Progress
	data, err := lifecycle.ReadMetadata(filepath.Join(home, "update-progress.json"))
	if errors.Is(err, os.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	return p, json.Unmarshal(data, &p)
}

// tracker publishes progress to a file. A write that fails is dropped: the
// update itself matters more than its progress bar.
type tracker struct {
	home string
	mu   sync.Mutex
	last time.Time
}

func (t *tracker) set(p Progress) {
	if t == nil || t.home == "" {
		return
	}
	data, err := json.Marshal(p)
	if err == nil {
		_ = lifecycle.WriteMetadata(filepath.Join(t.home, "update-progress.json"), append(data, '\n'))
	}
}

func (t *tracker) phase(phase string) { t.set(Progress{Phase: phase}) }

// bytes reports download progress at most four times a second.
func (t *tracker) bytes(done, total int64) {
	if t == nil {
		return
	}
	t.mu.Lock()
	due := time.Since(t.last) >= 250*time.Millisecond || (total > 0 && done >= total)
	if due {
		t.last = time.Now()
	}
	t.mu.Unlock()
	if due {
		t.set(Progress{Phase: PhaseDownloading, Done: done, Total: total})
	}
}
