package extres_test

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core/extres"
)

type stubProvider struct {
	kind string
	tag  string
}

func (s stubProvider) Kind() string                     { return s.kind }
func (s stubProvider) Validate(extres.Credential) error { return nil }
func (s stubProvider) Inspect(context.Context, string) (extres.Credential, error) {
	return extres.Credential{Kind: s.kind}, nil
}
func (s stubProvider) Verify(context.Context, extres.Credential) (extres.Observation, error) {
	return extres.Observation{Verdict: extres.VerdictPresent}, nil
}
func (s stubProvider) Bury(context.Context, extres.Credential, extres.MutateHooks) (extres.GraveArtifact, error) {
	return extres.GraveArtifact{}, nil
}
func (s stubProvider) Resurrect(context.Context, extres.Credential, extres.GraveArtifact, extres.MutateHooks) error {
	return nil
}
func (s stubProvider) Purge(context.Context, extres.Credential, extres.GraveArtifact, extres.MutateHooks) error {
	return nil
}

type policyProvider struct {
	stubProvider
	requireElapsed bool
	excludeRecycle bool
}

func (p policyProvider) RequireElapsedRetention() bool { return p.requireElapsed }
func (p policyProvider) ExcludeFromRecycleMode() bool  { return p.excludeRecycle }

func TestVerdictString(t *testing.T) {
	cases := []struct {
		v    extres.Verdict
		want string
	}{
		{extres.VerdictPresent, "present"},
		{extres.VerdictNotFound, "not-found"},
		{extres.VerdictIdentity, "identity"},
		{extres.VerdictReclaimed, "reclaimed"},
		{extres.VerdictBusy, "busy"},
		{extres.VerdictUnavailable, "unavailable"},
		{extres.Verdict(99), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.v.String(); got != tc.want {
			t.Fatalf("Verdict(%d).String() = %q, want %q", tc.v, got, tc.want)
		}
	}
}

func TestRegistryLookupOrderAndBoundaries(t *testing.T) {
	var nilReg *extres.Registry
	if _, ok := nilReg.Get("container"); ok {
		t.Fatal("nil registry returned a provider")
	}
	if kinds := nilReg.Kinds(); len(kinds) != 0 {
		t.Fatalf("nil registry kinds = %v", kinds)
	}
	if _, ok := nilReg.RetentionPolicyFor("container"); ok {
		t.Fatal("nil registry returned a retention policy")
	}

	empty := extres.NewRegistry(nil, stubProvider{kind: ""})
	if len(empty.Kinds()) != 0 {
		t.Fatalf("empty registry kept ignored entries: %v", empty.Kinds())
	}
	if _, ok := empty.Get(""); ok {
		t.Fatal("empty kind lookup succeeded")
	}

	reg := extres.NewRegistry(
		stubProvider{kind: "zeta", tag: "first"},
		policyProvider{stubProvider: stubProvider{kind: "alpha", tag: "p"}, requireElapsed: true, excludeRecycle: true},
		stubProvider{kind: "zeta", tag: "second"},
	)
	wantKinds := []string{"alpha", "zeta"}
	if got := reg.Kinds(); !slices.Equal(got, wantKinds) {
		t.Fatalf("Kinds() = %v, want %v", got, wantKinds)
	}
	p, ok := reg.Get("zeta")
	if !ok || p.(stubProvider).tag != "second" {
		t.Fatalf("expected later registration to win, got %#v ok=%v", p, ok)
	}
	if _, ok := reg.Get("missing"); ok {
		t.Fatal("missing kind lookup succeeded")
	}
	rp, ok := reg.RetentionPolicyFor("alpha")
	if !ok || !rp.RequireElapsedRetention() || !rp.ExcludeFromRecycleMode() {
		t.Fatalf("RetentionPolicyFor(alpha) = %#v ok=%v", rp, ok)
	}
	if _, ok := reg.RetentionPolicyFor("zeta"); ok {
		t.Fatal("non-policy provider reported RetentionPolicy")
	}
}

func TestGraveArtifactJSONRoundTrip(t *testing.T) {
	in := extres.GraveArtifact{
		Path:         "/grave/volume.tar",
		Digest:       "abc123",
		Size:         4096,
		HolderID:     "holder-1",
		EventStarted: "2026-11-28T10:00:00.123456789Z",
		Metadata:     json.RawMessage(`{"phase":"removed"}`),
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out extres.GraveArtifact
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Path != in.Path || out.Digest != in.Digest || out.Size != in.Size ||
		out.HolderID != in.HolderID || out.EventStarted != in.EventStarted ||
		string(out.Metadata) != string(in.Metadata) {
		t.Fatalf("round trip mismatch:\n got: %+v\nwant: %+v", out, in)
	}
}
