package hook

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExitCodeResponseOnlyDeniesPreToolUse(t *testing.T) {
	t.Parallel()
	for _, event := range []string{"PreToolUse", "PostToolUse", "SessionStart", "Context", "Stop", "SessionEnd", ""} {
		for _, deny := range []bool{true, false} {
			data, err := EncodeExitCodeResponse(Event{Name: event}, Result{Deny: deny, Reason: "use obit rm 中文", Context: "never delivered by this bridge"})
			if err != nil {
				t.Fatal(err)
			}
			if event == "PreToolUse" && deny {
				if string(data) != "{\"version\":1,\"event\":\"PreToolUse\",\"deny\":true,\"reason\":\"use obit rm 中文\"}\n" {
					t.Fatal(string(data))
				}
			} else if string(data) != "{}\n" {
				t.Fatalf("%s emitted non-neutral %s", event, data)
			}
		}
	}
	data, err := EncodeExitCodeResponse(Event{Name: "PreToolUse"}, Result{Deny: true, Reason: strings.Repeat("文", 3000)})
	var reply struct{ Reason string }
	if err != nil || json.Unmarshal(data, &reply) != nil || len(reply.Reason) > maxContext {
		t.Fatalf("unbounded reason: %s %v", data, err)
	}
	data, err = EncodeExitCodeResponse(Event{Name: "PreToolUse"}, Result{Deny: true})
	if err != nil || string(data) != "{}\n" {
		t.Fatal("empty denial")
	}
}
