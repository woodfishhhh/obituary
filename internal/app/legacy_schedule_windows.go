package app

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os/user"
	"reflect"
	"strings"
	"time"
)

// The scheduler exports SIDs and omits defaults. Recognize only the complete
// known export (or the original template), not a subset of parsed fields.
const fallbackExport = `<Task version="1.4" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
<RegistrationInfo><Description>Obituary: buries items whose lifespan ran out and purges the graveyard. Runs hidden every 5 minutes.</Description><URI>\%s</URI></RegistrationInfo>
<Principals><Principal id="Author"><UserId>%s</UserId><LogonType>InteractiveToken</LogonType></Principal></Principals>
<Settings><DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries><StopIfGoingOnBatteries>false</StopIfGoingOnBatteries><ExecutionTimeLimit>PT30M</ExecutionTimeLimit><Hidden>true</Hidden><MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy><StartWhenAvailable>true</StartWhenAvailable><IdleSettings><StopOnIdleEnd>false</StopOnIdleEnd><RestartOnIdle>false</RestartOnIdle></IdleSettings><UseUnifiedSchedulingEngine>true</UseUnifiedSchedulingEngine></Settings>
<Triggers><TimeTrigger><StartBoundary>%s</StartBoundary><Repetition><Interval>PT5M</Interval></Repetition></TimeTrigger><LogonTrigger><UserId>%s</UserId></LogonTrigger></Triggers>
<Actions Context="Author"><Exec><Command>%s</Command><Arguments>%s</Arguments></Exec></Actions></Task>`

func generatedFallback(definition, daemon, home, config string) bool {
	const localTime = "2006-01-02T15:04:05"
	var definitionTime struct {
		Start string `xml:"Triggers>TimeTrigger>StartBoundary"`
	}
	raw := strings.TrimSpace(strings.TrimPrefix(definition, `<?xml version="1.0" encoding="UTF-16"?>`))
	if err := xml.Unmarshal([]byte(raw), &definitionTime); err != nil {
		return false
	}
	at, err := time.Parse(localTime, definitionTime.Start)
	if err != nil || at.Format(localTime) != definitionTime.Start {
		return false
	}
	u, err := user.Current()
	if err != nil {
		return false
	}
	original := taskXML(daemon, home, u.Username, at)
	_, original, _ = strings.Cut(original, "?>")
	if sameTaskXML(raw, original) {
		return true
	}
	args := fmt.Sprintf(`--once --home "%s" --config-dir "%s"`, home, config)
	exported := fmt.Sprintf(fallbackExport, xmlText(TaskName), xmlText(u.Uid), definitionTime.Start, xmlText(u.Username), xmlText(daemon), xmlText(args))
	return sameTaskXML(raw, exported)
}

func sameTaskXML(a, b string) bool {
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
	for {
		x, err := next(left)
		y, other := next(right)
		if err != nil || other != nil {
			return errors.Is(err, io.EOF) && errors.Is(other, io.EOF)
		}
		if !reflect.DeepEqual(x, y) {
			return false
		}
	}
}
