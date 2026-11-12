package trust

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	untrustcmd "github.com/woodfishhhh/obituary/internal/cli/untrust"
	"github.com/woodfishhhh/obituary/internal/rules"
)

func TestTrustRejectsChangedExtendedRulesFile(t *testing.T) {
	testenv.WindowsOnly(t, "Win32 extended paths and Windows namespace normalization")
	for _, operation := range []string{"trust", "untrust"} {
		t.Run(operation, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			path := filepath.Join(repo, ".obitrules")
			clitest.WriteFixture(t, path, "*.log 1d\n")
			trust := rules.TrustStore{Path: filepath.Join(store.Home, "trusted_rules.json")}
			if operation == "untrust" {
				if _, err := trust.Trust(path, ""); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(trust.Path)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			cmd := NewCmd()
			args := []string{`\\?\` + path + ".", "--yes"}
			if operation == "untrust" {
				cmd, args = untrustcmd.NewCmd(), args[:1]
			}
			if out, _, err := clitest.Execute(t, cmd, args...); err == nil {
				t.Fatalf("rules command accepted changed extended name: %s", out)
			}
			after, err := os.ReadFile(trust.Path)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("rules command changed neighboring trust key")
			}
		})
	}
}

func TestTrustNormalizesExtendedRulesKey(t *testing.T) {
	testenv.WindowsOnly(t, "Win32 extended paths and Windows namespace normalization")
	clitest.IsolateRunner(t)
	repo, store := clitest.CommandDir(t)
	path := filepath.Join(repo, ".obitrules")
	clitest.WriteFixture(t, path, "*.log 1d\n")
	if out, _, err := clitest.Execute(t, NewCmd(), `\\?\`+path, "--yes"); err != nil {
		t.Fatalf("trust: %s %v", out, err)
	}
	trust := rules.TrustStore{Path: filepath.Join(store.Home, "trusted_rules.json")}
	if state, err := trust.Check(path); err != nil || state.Status != rules.Trusted {
		t.Fatalf("extended rules key differs from ordinary key: %+v %v", state, err)
	}
	if out, _, err := clitest.Execute(t, untrustcmd.NewCmd(), `\\?\`+path); err != nil {
		t.Fatalf("untrust: %s %v", out, err)
	}
	if state, err := trust.Check(path); err != nil || state.Status != rules.Untrusted {
		t.Fatalf("extended untrust missed ordinary key: %+v %v", state, err)
	}
}
