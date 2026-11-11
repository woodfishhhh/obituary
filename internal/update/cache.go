// Package update checks public release metadata and keeps it locally.
package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

const Interval = 24 * time.Hour

type Release struct {
	Version     string    `json:"version"`
	Name        string    `json:"name,omitempty"`
	Notes       string    `json:"notes,omitempty"`
	URL         string    `json:"url,omitempty"`
	PublishedAt time.Time `json:"publishedAt,omitzero"`
}

type Cache struct {
	CheckedAt time.Time `json:"checkedAt,omitzero"`
	ETag      string    `json:"etag,omitempty"`
	Latest    Release   `json:"latest,omitzero"`
	LastError string    `json:"lastError,omitempty"`
}

func Read(home string) (Cache, error) {
	data, err := core.ReadMetadata(filepath.Join(home, "update.json"))
	if errors.Is(err, os.ErrNotExist) {
		return Cache{}, nil
	}
	if err != nil {
		return Cache{}, err
	}
	var state Cache
	if err := json.Unmarshal(data, &state); err != nil {
		return Cache{}, fmt.Errorf("read update cache: %w", err)
	}
	if state.Latest.Version != "" && (!stable(state.Latest.Version) || state.Latest.URL != "" && !releaseURL(state.Latest.URL)) {
		return Cache{}, fmt.Errorf("read update cache: invalid release metadata")
	}
	return state, nil
}

func save(home string, state Cache) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return core.WriteMetadata(filepath.Join(home, "update.json"), append(data, '\n'))
}
