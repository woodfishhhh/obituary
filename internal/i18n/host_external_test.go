package i18n_test

import (
	"encoding/json"
	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"os"
	"strings"
	"testing"
)

func TestRegisteredHostDescriptionsHaveMessageIDs(t *testing.T) {
	data, err := os.ReadFile("messages/active.en.json")
	if err != nil {
		t.Fatal(err)
	}
	var messages map[string]string
	if err := json.Unmarshal(data, &messages); err != nil {
		t.Fatal(err)
	}
	hostMessageIDs := map[string]string{}
	for k, v := range messages {
		if strings.HasPrefix(k, "cli.host.") {
			hostMessageIDs[v] = k
		}
	}
	check := func(name string, c hook.Capabilities) {
		t.Helper()
		for _, field := range []hook.Support{c.DeleteGuard, c.GraveNotice, c.WriteObservation, c.SessionStart, c.Stop, c.SessionEnd, c.Context, c.MCP, c.Skill} {
			for _, txt := range []string{field.Detail, field.Verification.Evidence} {
				if txt != "" && hostMessageIDs[txt] == "" {
					t.Errorf("%s: description missing from catalog: %q", name, txt)
				}
			}
		}
	}
	for _, a := range integrate.Agents() {
		check(a.Name, a.Capabilities)
	}
	for _, a := range hook.Agents() {
		if _, ok := integrate.LookupAgent(a.Name); !ok {
			check(a.Name, a.Capabilities)
		}
	}
}
