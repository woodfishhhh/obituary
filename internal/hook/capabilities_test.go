package hook

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExistingAgentCapabilities(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"claude", "codex"} {
		agent, ok := Lookup(name)
		if !ok {
			t.Fatalf("missing agent %q", name)
		}
		for _, support := range []Support{agent.Capabilities.DeleteGuard, agent.Capabilities.WriteObservation,
			agent.Capabilities.SessionStart, agent.Capabilities.Stop, agent.Capabilities.SessionEnd,
			agent.Capabilities.Context, agent.Capabilities.GraveNotice, agent.Capabilities.MCP, agent.Capabilities.Skill} {
			if support.Level == Unsupported {
				t.Fatalf("missing %s capability", name)
			}
		}
	}
}

func TestCapabilitySupportRoundTrip(t *testing.T) {
	t.Parallel()
	for _, support := range []Support{{}, {Level: Supported}, {Level: Limited, Detail: "project scope only"}} {
		data, err := json.Marshal(support)
		var got Support
		if err != nil || json.Unmarshal(data, &got) != nil || got != support {
			t.Fatalf("capability changed: %s %+v %v", data, got, err)
		}
	}
}

func TestSupportDoesNotImplyHostVerification(t *testing.T) {
	t.Parallel()
	target := Target{Host: "Example CLI", Distribution: "CN / Windows", Version: "1.2.3", Scope: "project"}
	for _, level := range []SupportLevel{Supported, Limited, Unsupported} {
		s := Support{Level: level, Target: target}
		if s.ActuallyVerified() {
			t.Fatal("support confused with verification")
		}
		data, err := json.Marshal(s)
		if err != nil || !strings.Contains(string(data), `"actuallyVerified":false`) {
			t.Fatal("JSON lacks separate verification state", string(data), err)
		}
		s.Verification = Verification{Target: target}
		if s.ActuallyVerified() {
			t.Fatal("missing evidence accepted")
		}
		s.Verification.Evidence = "isolated local host receipt: pre deny delivered; original command did not execute"
		if !s.ActuallyVerified() {
			t.Fatal("complete host evidence not exposed")
		}
		data, err = json.Marshal(s)
		var roundtrip Support
		if err != nil || json.Unmarshal(data, &roundtrip) != nil || roundtrip != s {
			t.Fatalf("lost target/evidence: %s %v", data, err)
		}
		s.Verification.Target.Version = ""
		if s.ActuallyVerified() {
			t.Fatal("unspecified version counted as verified")
		}
	}
}
