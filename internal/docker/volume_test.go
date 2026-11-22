package docker

import (
	"maps"
	"strings"
	"testing"
	"time"
)

func testVolume() Volume {
	return Volume{Name: "cache", Driver: "local", Scope: "local", CreatedAt: time.Unix(123, 0).UTC(), Target: Target{Context: "desktop-linux", Endpoint: "unix:///docker.sock", DaemonID: "engine", OSType: "linux"}, Labels: map[string]string{"project": "tests"}}
}

func TestVolumeIdentityAndRestoreOwnership(t *testing.T) {
	v := testVolume()
	if err := v.Check(); err != nil {
		t.Fatal(err)
	}
	otherContext := v
	otherContext.Target.Context = "another-context"
	if v.Same(otherContext) {t.Fatal("volume identity accepted a different registration context")}
	for _, change := range []func(*Volume){func(x *Volume) { x.Name = "CACHE" }, func(x *Volume) { x.CreatedAt = x.CreatedAt.Add(time.Second) }, func(x *Volume) { x.Labels = map[string]string{"project": "other"} }, func(x *Volume) { x.Target.DaemonID = "other" }, func(x *Volume) { x.Driver = "plugin" }, func(x *Volume) { x.Options = map[string]string{"type": "tmpfs"} }} {
		x := v
		change(&x)
		if v.Same(x) {
			t.Fatalf("accepted different volume: %s", x.Name)
		}
	}
	for _, change := range []func(*Volume){func(x *Volume) { x.CreatedAt = time.Time{} }, func(x *Volume) { x.Target.OSType = "windows" }, func(x *Volume) { x.Driver = "plugin" }, func(x *Volume) { x.Scope = "global" }, func(x *Volume) { x.Name = "../wrong" }} {
		x := v
		change(&x)
		if x.Check() == nil {
			t.Fatal("unsupported volume accepted")
		}
	}
	opt := v
	opt.Options = map[string]string{"o": "secret-password"}
	if err := opt.Check(); err == nil || strings.Contains(err.Error(), "secret-password") {
		t.Fatalf("unsafe/redaction: %v", err)
	}
	labels, err := v.RestoreLabels(nil, "new-token")
	if err != nil || labels[RestoreLabel] != "new-token" || !maps.Equal(v.Labels, map[string]string{"project": "tests"}) {
		t.Fatal(labels, err)
	}
	restored := v
	restored.Labels = labels
	original := restored
	original.CreatedAt = original.CreatedAt.Add(-time.Second)
	for _, known := range []*Volume{nil, &original} {
		if _, err := restored.RestoreLabels(known, "next-token"); err == nil {
			t.Fatal("foreign label overwritten")
		}
	}
	again, err := restored.RestoreLabels(&restored, "next-token")
	if err != nil || len(again) != 2 || again[RestoreLabel] != "next-token" {
		t.Fatal(again, err)
	}
}
