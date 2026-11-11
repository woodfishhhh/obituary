package hook

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

type graveSession struct {
	Version int           `json:"obituaryGraveNotices"`
	Seen    []int64       `json:"seen,omitempty"`
	Pending []graveNotice `json:"pending,omitempty"`
}

func graveSessionPath(home string, ev Event) string {
	key := sha256.Sum256([]byte(ev.Host + "\x00" + ev.Session))
	return filepath.Join(home, "sessions", fmt.Sprintf("grave-notices-%x.json", key))
}

func readGraveSession(home string, ev Event) graveSession {
	var state graveSession
	data, err := os.ReadFile(graveSessionPath(home, ev))
	if err == nil {
		_ = json.Unmarshal(data, &state)
	}
	return state
}

func writeGraveSession(home string, ev Event, pending []graveNotice, seen []int64) {
	path := graveSessionPath(home, ev)
	state := readGraveSession(home, ev)
	state.Version = 1
	for _, id := range seen {
		if !slices.Contains(state.Seen, id) {
			state.Seen = append(state.Seen, id)
		}
	}
	for _, n := range pending {
		if !slices.Contains(state.Seen, n.ID) && !slices.ContainsFunc(state.Pending, func(p graveNotice) bool { return p.ID == n.ID }) {
			state.Pending = append(state.Pending, n)
		}
	}
	state.Pending = slices.DeleteFunc(state.Pending, func(n graveNotice) bool { return slices.Contains(state.Seen, n.ID) })
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return
	}
	defer root.Close()
	name := filepath.Base(path)
	if err := root.WriteFile(name+".tmp", data, 0600); err == nil {
		_ = root.Rename(name+".tmp", name)
	}
}
