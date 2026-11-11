package doctor

import (
	"fmt"
	"strings"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func diagnoseIdentities(home string) DoctorCheck {
	svc := &core.Service{Store: core.Store{Home: home}, FS: winfs.FS{}}
	events, err := svc.RegistrationLocationDiagnostics()
	c := DoctorCheck{Name: "registration-identities", Status: "PASS", Detail: "recorded identities have no location errors", Errors: len(events)}
	if err != nil {
		c.Status, c.Detail = "WARN", err.Error()
	} else if len(events) > 0 {
		var details []string
		for _, event := range events {
			details = append(details, fmt.Sprintf("#%d: %s", event.ID, event.Detail))
		}
		c.Status, c.Detail = "WARN", strings.Join(details, "; ")
	}
	return c
}
