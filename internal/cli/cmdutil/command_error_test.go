package cmdutil

import (
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCobraErrorsKeepMachineAndEnglishOutput(t *testing.T) {
	for _, tc := range []struct {
		args string
		id   string
	}{
		{"missing", "unknown_command"}, {"sample --bad", "unknown_flag"}, {"sample", "argument_count"}, {"sample target", "required_flags"}, {"sample target --number", "missing_value"}, {"sample target --number=no", "invalid_value"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			root := &cobra.Command{Use: "obit", SilenceErrors: true, SilenceUsage: true}
			leaf := &cobra.Command{Use: "sample", Args: cobra.ExactArgs(1), Run: func(*cobra.Command, []string) {}}
			leaf.Flags().Int("number", 0, "")
			if err := leaf.MarkFlagRequired("number"); err != nil {
				t.Fatal(err)
			}
			root.AddCommand(leaf)
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			argv := strings.Fields(tc.args)
			root.SetArgs(argv)
			cmd, err := root.ExecuteC()
			if err == nil {
				t.Fatal("expected Cobra error")
			}
			id, detail := commandErrorMessage(err)
			if id != "cli.cobra."+tc.id || detail == "" {
				t.Fatalf("%s: %s %q", err, id, detail)
			}
			t.Setenv("OBIT_LANG", "en")
			if got := CommandError(cmd, argv, err); got != err.Error() {
				t.Fatal(got)
			}
			t.Setenv("OBIT_LANG", "zh-CN")
			if got := CommandError(cmd, append(argv, "--json"), err); got != err.Error() {
				t.Fatal(got)
			}
			if got := CommandError(cmd, argv, err); got == err.Error() {
				t.Fatalf("missing localized explanation: %s", got)
			}
		})
	}
}
