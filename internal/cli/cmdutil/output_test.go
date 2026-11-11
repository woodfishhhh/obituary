//go:build windows

package cmdutil_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	addcmd "github.com/woodfishhhh/obituary/internal/cli/add"
	burycmd "github.com/woodfishhhh/obituary/internal/cli/bury"
	cancelcmd "github.com/woodfishhhh/obituary/internal/cli/cancel"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	extendcmd "github.com/woodfishhhh/obituary/internal/cli/extend"
	resurrectcmd "github.com/woodfishhhh/obituary/internal/cli/resurrect"
	"github.com/woodfishhhh/obituary/internal/core"
)

func TestOtherJSONMutationCommandsReportItemErrors(t *testing.T) {
	for _, command := range []func() *cobra.Command{addcmd.NewCmd, extendcmd.NewCmd, cancelcmd.NewCmd, burycmd.NewCmd, resurrectcmd.NewCmd} {
		t.Run(command().Name(), func(t *testing.T) {
			clitest.IsolateRunner(t)
			clitest.CommandDir(t)
			cmd := command()
			args := []string{"missing"}
			if cmd.Name() == "add" || cmd.Name() == "extend" {
				args = append(args, "1d")
			}
			out, _, err := clitest.Execute(t, cmd, append(args, "--json")...)
			if !errors.Is(err, cmdutil.ErrReported) || !json.Valid([]byte(out)) || !strings.Contains(out, `"error"`) {
				t.Fatalf("err=%v output=%s", err, out)
			}
		})
	}
}

func TestGraveFailureOutputIdentifiesReference(t *testing.T) {
	clitest.IsolateRunner(t)
	repo, _ := clitest.CommandDir(t)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	var graves []core.Burial
	for _, name := range []string{"good.txt", "bad.txt"} {
		path := filepath.Join(repo, name)
		clitest.WriteFixture(t, path, "keep")
		rows, err := svc.Bury([]string{path}, false, "")
		if err != nil || core.GraveFailed(rows) {
			t.Fatal(rows, err)
		}
		graves = append(graves, *rows[0].Burial)
	}
	clitest.WriteFixture(t, filepath.Join(filepath.Dir(graves[1].Grave), "foreign.txt"), "keep")
	rows, err := svc.Purge(true, false)
	if err != nil || len(rows) != 2 || rows[0].Error != "" || rows[1].Error == "" {
		t.Fatal(rows, err)
	}
	for _, asJSON := range []bool{false, true} {
		cmd := &cobra.Command{RunE: func(*cobra.Command, []string) error {
			return cmdutil.ReportGraves(rows, asJSON, func(r core.GraveResult) string { return r.Action + " " + r.Ref })
		}}
		out, stderr, err := clitest.Execute(t, cmd)
		if !errors.Is(err, cmdutil.ErrReported) {
			t.Fatal(err)
		}
		if asJSON {
			want, err := json.MarshalIndent(rows, "", "  ")
			if err != nil || out != string(want)+"\n" {
				t.Fatalf("JSON grave receipt changed: %v", err)
			}
		} else if !strings.Contains(stderr, fmt.Sprintf("ref=%q", rows[1].Ref)) || !strings.Contains(stderr, fmt.Sprintf("id=%d", graves[1].ID)) || !strings.Contains(stderr, fmt.Sprintf("path=%s:", graves[1].Path)) || !strings.Contains(out, "purged ") {
			t.Fatalf("grave failure omitted reference identity: stdout=%s stderr=%s", out, stderr)
		}
	}
}

func TestCommittedRejectsMixedErrorTree(t *testing.T) {
	t.Parallel()
	hard := errors.New("ledger commit failed")
	audit := &core.AuditError{Err: errors.New("audit unavailable")}
	for _, err := range []error{errors.Join(audit, hard), fmt.Errorf("batch: %w", errors.Join(hard, audit))} {
		if !errors.Is(cmdutil.Committed(err), hard) {
			t.Fatalf("audit warning swallowed hard failure: %v", err)
		}
	}
	if err := cmdutil.Committed(errors.Join(audit, fmt.Errorf("wrapped: %w", audit))); err != nil {
		t.Fatal(err)
	}
}

func TestPendingMutationCommandsFailInTextAndJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, operation := range []string{"extend", "spare", "add"} {
		for _, asJSON := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%v", operation, asJSON), func(t *testing.T) {
				svc, path, p := clitest.PendingCommandBurial(t, true)
				before, _ := svc.Store.Read()
				var cmd *cobra.Command
				var args []string
				ref := fmt.Sprint(p.Burial.ID)
				switch operation {
				case "extend":
					cmd, args = extendcmd.NewCmd(), []string{ref, "30d"}
				case "spare":
					cmd, args = cancelcmd.NewCmd(), []string{ref}
				case "add":
					clitest.WriteFixture(t, path, "replacement")
					cmd, args = addcmd.NewCmd(), []string{path, "30d"}
				}
				if asJSON {
					args = append(args, "--json")
				}
				out, stderr, err := clitest.Execute(t, cmd, args...)
				if err == nil || !strings.Contains(out+stderr, "is being buried") {
					t.Fatalf("command reported success or no recovery advice: %s %s, %v", out, stderr, err)
				}
				if asJSON {
					var rows []core.Result
					if json.Unmarshal([]byte(out), &rows) != nil || !core.Failed(rows) {
						t.Fatalf("invalid JSON failure: %s", out)
					}
				}
				after, _ := svc.Store.Read()
				if !reflect.DeepEqual(before, after) {
					t.Fatal("failed CLI mutation changed state")
				}
			})
		}
	}
}
