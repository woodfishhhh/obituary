// Package extres defines the external-resource provider contract used by
// Obituary's lifecycle engine. It depends only on the Go standard library so
// resource adapters and core packages share one acyclic vocabulary.
package extres

import (
	"context"
	"encoding/json"
	"time"
)

// Verdict classifies a live verification check of a pinned external resource.
type Verdict uint8

const (
	// VerdictPresent means the pinned object exists with matching identity.
	VerdictPresent Verdict = iota
	// VerdictNotFound means the pinned origin definitively reported the object absent.
	VerdictNotFound
	// VerdictIdentity means the object's immutable identity changed or is invalid.
	VerdictIdentity
	// VerdictReclaimed means the object was restarted or taken over outside Obituary.
	VerdictReclaimed
	// VerdictBusy means the object is actively referenced and cannot be safely buried.
	VerdictBusy
	// VerdictUnavailable means the external engine could not be reached or verified.
	VerdictUnavailable
)

// String returns the canonical lowercase label for a verdict.
func (v Verdict) String() string {
	switch v {
	case VerdictPresent:
		return "present"
	case VerdictNotFound:
		return "not-found"
	case VerdictIdentity:
		return "identity"
	case VerdictReclaimed:
		return "reclaimed"
	case VerdictBusy:
		return "busy"
	case VerdictUnavailable:
		return "unavailable"
	default:
		return "unknown"
	}
}

// Credential is the immutable, serializable identity of an external resource.
type Credential struct {
	Kind    string          `json:"kind"`
	Key     string          `json:"key"`
	Display string          `json:"display"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Observation describes the live state of a saved external resource.
type Observation struct {
	Verdict Verdict
	Active  bool
	Detail  string
	Updated *Credential
}

// GraveArtifact describes local recovery material produced when burying an
// external resource (for example a verified Docker volume archive).
type GraveArtifact struct {
	Path         string          `json:"path,omitempty"`
	Digest       string          `json:"digest,omitempty"`
	Size         int64           `json:"size,omitempty"`
	HolderID     string          `json:"holderId,omitempty"`
	EventStarted string          `json:"eventStarted,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
}

// MutateHooks bridges external provider mutations with lifecycle's durable
// intents and local grave storage without coupling providers to store locks.
type MutateHooks struct {
	BeforeSend      func(phase string, evidence json.RawMessage) error
	PrepareGraveDir func(id int64) (dir string, cleanup func(), err error)
}

// Provider is the minimal external resource lifecycle contract.
type Provider interface {
	Kind() string
	Validate(cred Credential) error
	Inspect(ctx context.Context, ref string) (Credential, error)
	Verify(ctx context.Context, saved Credential) (Observation, error)
	Bury(ctx context.Context, saved Credential, hooks MutateHooks) (GraveArtifact, error)
	Resurrect(ctx context.Context, saved Credential, grave GraveArtifact, hooks MutateHooks) error
	Purge(ctx context.Context, saved Credential, grave GraveArtifact, hooks MutateHooks) error
}

// RetentionPolicy is an optional provider capability declaring burial retention
// and recycle-mode rules for a resource kind.
type RetentionPolicy interface {
	RequireElapsedRetention() bool
	ExcludeFromRecycleMode() bool
}

// DiagnoseRequest scopes a provider health or recovery check.
type DiagnoseRequest struct {
	Home string
	Now  time.Time
}

// DoctorCheck is one diagnostic finding returned by a Doctor provider.
type DoctorCheck struct {
	Code    string
	Status  string
	Summary string
	Detail  string
}

// Doctor is an optional provider capability for availability and diagnostics.
type Doctor interface {
	Available(ctx context.Context) (blockedCode string, err error)
	Diagnose(ctx context.Context, req DiagnoseRequest) []DoctorCheck
}

// SessionWindow bounds automatic external-resource discovery for one session.
type SessionWindow struct {
	Project string
	Session string
	Started time.Time
	Ended   time.Time
}

// Candidate is an external resource proposed for lifespan registration.
type Candidate struct {
	Credential Credential
	Reason     string
	Lifespan   string
}

// Discoverer is an optional provider capability for session-scoped discovery.
type Discoverer interface {
	Discover(ctx context.Context, window SessionWindow) ([]Candidate, error)
}
