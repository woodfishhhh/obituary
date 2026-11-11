package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tc-hib/winres"
)

func TestCheckResourcesRejectsIncompleteOrStaleArtifacts(t *testing.T) {
	icon, err := brandIcon("../..")
	if err != nil {
		t.Fatal(err)
	}
	var ico bytes.Buffer
	if err := icon.SaveICO(&ico); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, want string
		change     func(*winres.ResourceSet) error
	}{
		{"complete", "", func(*winres.ResourceSet) error { return nil }},
		{"no-version", "version resource", func(rs *winres.ResourceSet) error {
			return rs.Set(winres.RT_VERSION, winres.ID(1), winres.LCIDDefault, []byte("broken"))
		}},
		{"old-version", "fixed version", func(rs *winres.ResourceSet) error {
			vi, err := versionInfo("../..", applications[0], "0.0.0-dev")
			if err == nil {
				rs.SetVersionInfo(*vi)
			}
			return err
		}},
		{"wrong-description", "FileDescription", func(rs *winres.ResourceSet) error {
			app := applications[0]
			app.description = "后台与托盘"
			vi, err := versionInfo("../..", app, "1.2.3-SNAPSHOT-abc123")
			if err == nil {
				rs.SetVersionInfo(*vi)
			}
			return err
		}},
		{"no-manifest", "manifest", func(rs *winres.ResourceSet) error {
			return rs.Set(winres.RT_MANIFEST, winres.ID(1), winres.LCIDDefault, []byte("broken"))
		}},
		{"no-icon", "icon", func(rs *winres.ResourceSet) error {
			return rs.Set(winres.RT_GROUP_ICON, appIconID, winres.LCIDNeutral, []byte("broken"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rs, err := resources("../..", applications[0], "1.2.3-SNAPSHOT-abc123", icon)
			if err != nil {
				t.Fatal(err)
			}
			if err := tc.change(rs); err != nil {
				t.Fatal(err)
			}
			err = checkResources("../..", rs, applications[0], "1.2.3-SNAPSHOT-abc123", ico.Bytes())
			if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}
