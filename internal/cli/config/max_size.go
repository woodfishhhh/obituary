package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/textutil"
)

var isTerminal = func() bool { return cmdutil.IsTerminal(os.Stdin) }

func applyMaxSize(cmd *cobra.Command, raw string, yes, dryRun bool) error {
	normalized, maxBytes, err := config.NormalizeByteSize(raw)
	if err != nil {
		return err
	}
	path, err := config.Path()
	if err != nil {
		return err
	}
	change, err := integrate.PlanGraveyardMaxSizeConfig(path, normalized)
	if err != nil {
		return err
	}
	svc, preview, err := previewMaxSizeEviction(maxBytes)
	if err != nil {
		return err
	}
	if dryRun {
		printMaxSizePreview(normalized, preview)
		return nil
	}
	overLimit := maxBytes > 0 && preview.BeforeBytes > maxBytes
	if overLimit && !yes {
		if err := confirmMaxSizeEviction(cmd, preview); err != nil {
			return err
		}
	}
	if overLimit && svc != nil {
		currentHash, err := svc.GraveyardSnapshotHash()
		if err != nil {
			return err
		}
		if currentHash != preview.GraveyardHash {
			return errors.New(cmdutil.Text("cli.config.max_size_graveyard_changed"))
		}
	}
	if err := integrate.Apply([]integrate.Change{change}); err != nil {
		return err
	}
	fmt.Printf("max_size = %q\n", normalized)
	if maxBytes <= 0 || svc == nil {
		return nil
	}
	svc.GraveyardMaxBytes = maxBytes
	ev, evErr := svc.EnforceGraveyardQuota(false)
	cmdutil.PrintQuotaEviction(os.Stderr, cmdutil.Localizer(), &ev, false)
	return cmdutil.Committed(evErr)
}

func previewMaxSizeEviction(maxBytes int64) (*lifecycle.Service, lifecycle.QuotaEviction, error) {
	if maxBytes <= 0 {
		return nil, lifecycle.QuotaEviction{}, nil
	}
	svc, err := cmdutil.Open()
	if err != nil {
		return nil, lifecycle.QuotaEviction{}, err
	}
	ev, err := svc.PreviewGraveyardQuotaEviction(maxBytes)
	return svc, ev, err
}

func printMaxSizePreview(normalized string, ev lifecycle.QuotaEviction) {
	fmt.Printf("max_size = %q\n", normalized)
	if ev.MaxBytes <= 0 || ev.BeforeBytes <= ev.MaxBytes {
		return
	}
	printEvictionSummary(ev)
}

func printEvictionSummary(ev lifecycle.QuotaEviction) {
	l := cmdutil.Localizer()
	fmt.Fprintln(os.Stderr, l.Text("cli.config.max_size_preview", map[string]any{
		"Used":  textutil.Bytes(ev.BeforeBytes),
		"Limit": textutil.Bytes(ev.MaxBytes),
		"Count": ev.PurgedCount,
		"Freed": textutil.Bytes(ev.FreedBytes),
		"After": textutil.Bytes(ev.AfterBytes),
	}))
	for _, item := range ev.Items {
		fmt.Fprintf(os.Stderr, "  #%d  %s  (%s)\n", item.ID, item.Path, textutil.Bytes(item.Size))
	}
	if ev.FailedCount > 0 {
		fmt.Fprintln(os.Stderr, l.Text("cli.quota.eviction_partial_failed", map[string]any{
			"Failed": ev.FailedCount,
		}))
	}
}

func confirmMaxSizeEviction(cmd *cobra.Command, ev lifecycle.QuotaEviction) error {
	printEvictionSummary(ev)
	if !isTerminal() {
		return errors.New(cmdutil.Text("cli.config.max_size_not_a_terminal"))
	}
	fmt.Fprint(os.Stderr, cmdutil.Text("cli.config.max_size_confirm"))
	answer, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
		if err != nil && answer == "" {
			return fmt.Errorf("%w: %s", err, cmdutil.Text("cli.config.max_size_canceled"))
		}
		return errors.New(cmdutil.Text("cli.config.max_size_canceled"))
	}
	return nil
}
