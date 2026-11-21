package app

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

const fallbackReceipt = "obitd.runner-task.xml"
const legacyFallbackSnapshot = "obitd.runner-before-migration.xml"

// The scheduler exports SIDs and omits defaults. Recognize only the complete
// known export (or the original template), not a subset of parsed fields.
const fallbackExport = `<Task version="1.4" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
<RegistrationInfo><Description>Obituary: buries items whose lifespan ran out and purges the graveyard. Runs hidden every 5 minutes.</Description><URI>\%s</URI></RegistrationInfo>
<Principals><Principal id="Author"><UserId>%s</UserId><LogonType>InteractiveToken</LogonType></Principal></Principals>
<Settings><DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries><StopIfGoingOnBatteries>false</StopIfGoingOnBatteries><ExecutionTimeLimit>PT30M</ExecutionTimeLimit><Hidden>true</Hidden><MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy><StartWhenAvailable>true</StartWhenAvailable><IdleSettings><StopOnIdleEnd>false</StopOnIdleEnd><RestartOnIdle>false</RestartOnIdle></IdleSettings><UseUnifiedSchedulingEngine>true</UseUnifiedSchedulingEngine></Settings>
<Triggers><TimeTrigger><StartBoundary>%s</StartBoundary><Repetition><Interval>PT5M</Interval></Repetition></TimeTrigger><LogonTrigger><UserId>%s</UserId></LogonTrigger></Triggers>
<Actions Context="Author"><Exec><Command>%s</Command><Arguments>%s</Arguments></Exec></Actions></Task>`

func generatedFallback(definition, daemon, home, config string) bool {
	return fallbackDifference(definition, daemon, home, config) == ""
}

func fallbackDifference(definition, daemon, home, config string) string {
	raw := strings.TrimSpace(strings.TrimPrefix(definition, `<?xml version="1.0" encoding="UTF-16"?>`))
	start := taskStartBoundary(raw)
	if start == "" {
		return "Triggers/TimeTrigger/StartBoundary"
	}
	u, err := user.Current()
	if err != nil {
		return err.Error()
	}
	original := taskXML(daemon, home, u.Username, time.Time{})
	original = strings.Replace(original, "<StartBoundary>0001-01-01T00:00:00</StartBoundary>", "<StartBoundary>"+xmlText(start)+"</StartBoundary>", 1)
	_, original, _ = strings.Cut(original, "?>")
	sourceDiff, sourceMatch := taskXMLDifference(original, raw, u)
	if sourceDiff == "" {
		return ""
	}
	args := fmt.Sprintf(`--once --home "%s" --config-dir "%s"`, home, config)
	exported := fmt.Sprintf(fallbackExport, xmlText(TaskName), xmlText(u.Uid), xmlText(start), xmlText(u.Username), xmlText(daemon), xmlText(args))
	diff, matched := taskXMLDifference(exported, raw, u)
	if diff == "" || matched >= sourceMatch {
		return diff
	}
	return sourceDiff
}

func taskStartBoundary(definition string) string {
	var task struct {
		Start string `xml:"Triggers>TimeTrigger>StartBoundary"`
	}
	_, body, found := strings.Cut(definition, "?>")
	if !found {
		body = definition
	}
	if err := xml.Unmarshal([]byte(body), &task); err != nil {
		return ""
	}
	return task.Start
}

func taskXMLDifference(a, b string, account *user.User) (string, int) {
	left, right := xml.NewDecoder(strings.NewReader(a)), xml.NewDecoder(strings.NewReader(b))
	next := func(d *xml.Decoder) (xml.Token, error) {
		for {
			token, err := d.Token()
			if data, ok := token.(xml.CharData); ok && strings.TrimSpace(string(data)) == "" {
				continue
			}
			return token, err
		}
	}
	var path []string
	for matched := 0; ; matched++ {
		x, err := next(left)
		y, other := next(right)
		if err != nil || other != nil {
			if errors.Is(err, io.EOF) && errors.Is(other, io.EOF) {
				return "", matched
			}
			return "invalid or incomplete XML: /" + strings.Join(path, "/"), matched
		}
		if !reflect.DeepEqual(x, y) && !sameTaskUser(x, y, path, account) {
			if found, ok := y.(xml.StartElement); ok {
				path = append(path, found.Name.Local)
			} else if want, ok := x.(xml.StartElement); ok {
				path = append(path, want.Name.Local)
			}
			return "/" + strings.Join(path, "/"), matched
		}
		switch node := x.(type) {
		case xml.StartElement:
			path = append(path, node.Name.Local)
		case xml.EndElement:
			path = path[:len(path)-1]
		}
	}
}

func sameTaskUser(x, y xml.Token, path []string, account *user.User) bool {
	if len(path) == 0 || path[len(path)-1] != "UserId" {
		return false
	}
	isAccount := func(t xml.Token) bool {
		v, ok := t.(xml.CharData)
		return ok && (strings.EqualFold(string(v), account.Username) || strings.EqualFold(string(v), account.Uid))
	}
	return isAccount(x) && isAccount(y)
}

// The original export remains the rollback source; never replace it on retry.
func snapshotFallback(home, current string) error {
	path := filepath.Join(home, legacyFallbackSnapshot)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if errors.Is(err, os.ErrExist) {
		before, err := core.ReadMetadata(path)
		if err == nil && string(before) != current {
			return errors.New("fallback differs from the saved migration snapshot; preserved")
		}
		return err
	}
	if err != nil {
		return err
	}
	_, writeErr := io.WriteString(f, current)
	return errors.Join(writeErr, f.Sync(), f.Close())
}

func restoreFallbackSnapshot(home, daemon, config string) error {
	saved, err := core.ReadMetadata(filepath.Join(home, legacyFallbackSnapshot))
	if err != nil {
		return err
	}
	if field := fallbackDifference(string(saved), daemon, home, config); field != "" {
		return fmt.Errorf("preserving unrecognized snapshot: %s", field)
	}
	current, err := registeredTaskXML(TaskName)
	if errors.Is(err, os.ErrNotExist) {
		if err := registerTask(TaskName, string(saved)); err != nil {
			return err
		}
		current, err = registeredTaskXML(TaskName)
	}
	if err != nil {
		return err
	}
	if current != string(saved) {
		return errors.New("fallback differs from rollback snapshot; not replacing it")
	}
	return nil
}
