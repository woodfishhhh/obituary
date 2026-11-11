package hook

import "encoding/json"

// Target bounds a protocol claim. Empty values mean unspecified, not all hosts,
// releases or scopes. Version can name a release range or a pinned source commit.
type Target struct {
	Host         string `json:"host,omitempty"`
	Distribution string `json:"distribution,omitempty"`
	Version      string `json:"version,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// Verification records actual host acceptance, not fixtures or an installation
// receipt. Evidence should identify the recorded version, scope and outcome.
type Verification struct {
	Target   Target `json:"target"`
	Evidence string `json:"evidence"`
}

// Support describes protocol coverage independently of local installation and
// actual host acceptance. Empty Verification means no actual-host evidence.
type Support struct {
	Level        SupportLevel `json:"level"`
	Detail       string       `json:"detail,omitempty"`
	Target       Target       `json:"target"` // zero value inherits Capabilities.Target
	Verification Verification `json:"verification"`
}

type SupportLevel string

const (
	Unsupported SupportLevel = ""
	Supported   SupportLevel = "supported"
	Limited     SupportLevel = "limited"
)

// Capabilities is shared by hook adapters and installation descriptors. Installed
// and current state are separate runtime inspections, never implied by support.
type Capabilities struct {
	Target           Target  `json:"target"`
	GraveNotice      Support `json:"graveNotice"`
	DeleteGuard      Support `json:"deleteGuard"`
	WriteObservation Support `json:"writeObservation"`
	SessionStart     Support `json:"sessionStart"`
	Stop             Support `json:"stop"`
	SessionEnd       Support `json:"sessionEnd"`
	Context          Support `json:"context"`
	MCP              Support `json:"mcp"`
	Skill            Support `json:"skill"`
}

// ActuallyVerified excludes incomplete records; callers cannot turn a support
// level or an installed file into a successful host acceptance claim.
func (s Support) ActuallyVerified() bool {
	v := s.Verification
	return v.Evidence != "" && v.Target.Host != "" && v.Target.Version != "" && v.Target.Scope != ""
}

func (s Support) MarshalJSON() ([]byte, error) {
	type fields Support
	return json.Marshal(struct {
		fields
		ActuallyVerified bool `json:"actuallyVerified"`
	}{fields(s), s.ActuallyVerified()})
}
