package rules_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	rulescmd "github.com/woodfishhhh/obituary/internal/cli/rules"
	"github.com/woodfishhhh/obituary/internal/rules"
)

func TestLegacyTrustFirstRunMessages(t *testing.T) {
	clitest.IsolateRunner(t)
	repo, store := clitest.CommandDir(t)
	clitest.WriteFixture(t, filepath.Join(repo, rules.FileName), "notes/** 1m\n")
	paths := []string{filepath.Join(repo, "notes", "a.md"), filepath.Join(repo, "notes", "b.md")}
	for _, path := range paths {
		clitest.WriteFixture(t, path, "private")
	}
	clitest.WriteFixture(t, filepath.Join(store.Home, "trusted_rules.json"), `{"version":1,"trusted":{}}`)
	const warning = "trust store format was upgraded; review the rules and run obit trust again"
	for _, command := range []string{"check", "apply"} {
		for _, format := range []string{"text", "json"} {
			t.Run(command+"/"+format, func(t *testing.T) {
				args := append([]string{command}, paths...)
				if format == "json" {
					args = append(args, "--json")
				}
				out, stderr, err := clitest.Execute(t, rulescmd.NewCmd(), args...)
				if err != nil || strings.Count(out, warning) != 1 {
					t.Fatalf("legacy trust warning missing or duplicated: %s %s %v", out, stderr, err)
				}
				if format == "json" {
					var rows []auto.Decision
					if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) != 2 || rows[0].Action != auto.Skipped || rows[1].Action != auto.Skipped {
						t.Fatalf("legacy rules changed registration: %s %v", out, err)
					}
				}
			})
		}
	}
	st, err := store.ReadSnapshot()
	if err != nil || len(st.Items) != 0 {
		t.Fatalf("legacy rules registered private files: %+v %v", st, err)
	}
}
