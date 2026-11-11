package uninstall

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

var sessionName = regexp.MustCompile(`^(?:[a-f0-9]{12}-[A-Za-z0-9_-]*\.json|[a-f0-9]{16}\.finish|grave-notices-[a-f0-9]{64}\.json(?:\.lock|\.tmp)?)$`)

func sessionFiles(root *os.Root) ([]string, error) {
	dir, err := root.Open("sessions")
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		name := "sessions/" + entry.Name()
		info, err := root.Lstat(name)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || !sessionName.MatchString(entry.Name()) {
			return nil, fmt.Errorf("data retained: unrecognized session file %s", name)
		}
		data, err := root.ReadFile(name)
		if err != nil {
			return nil, err
		}
		if !ownedSession(entry.Name(), data) {
			return nil, fmt.Errorf("data retained: unrecognized session contents %s", name)
		}
		names = append(names, name)
	}
	return names, nil
}

func ownedSession(name string, data []byte) bool {
	if strings.HasPrefix(name, "grave-notices-") {
		if strings.HasSuffix(name, ".lock") {
			return len(data) == 0
		}
		var notice struct {
			Version int `json:"obituaryGraveNotices"`
		}
		return json.Unmarshal(data, &notice) == nil && notice.Version == 1
	}
	if strings.HasSuffix(name, ".finish") {
		return len(data) == 0
	}
	var snapshot struct {
		Project string
		At      time.Time
	}
	return json.Unmarshal(data, &snapshot) == nil && snapshot.Project != "" && !snapshot.At.IsZero()
}

func removeSessions(root *os.Root) error {
	names, err := sessionFiles(root)
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := root.Remove(name); err != nil {
			return err
		}
	}
	return nil
}
