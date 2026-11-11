package cmdutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/textutil"
)

func OnlyAuditErrors(err error) bool {
	return core.OnlyAuditErrors(err)
}

func HumanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func ReportGraves(results []core.GraveResult, asJSON bool, describe func(core.GraveResult) string) error {
	if asJSON {
		if err := PrintJSON(results); err != nil {
			return err
		}
	} else {
		for _, r := range results {
			printGraveResult(r, describe)
		}
	}
	if core.GraveFailed(results) {
		return ErrReported
	}
	return nil
}

func printGraveResult(r core.GraveResult, describe func(core.GraveResult) string) {
	if r.Warning != "" {
		fmt.Fprintln(os.Stderr, "warning:", r.Warning)
	}
	if r.Error == "" {
		fmt.Println(describe(r))
		return
	}
	label := fmt.Sprintf("ref=%q", r.Ref)
	if r.Burial != nil {
		label += fmt.Sprintf(" id=%d path=%s", r.Burial.ID, r.Burial.Path)
	}
	if r.Action == core.ActionResurrected && r.Path != "" {
		fmt.Println(describe(r))
	}
	if r.Action == core.ActionNotExecuted {
		fmt.Fprintln(os.Stderr, r.Ref+":", r.Error)
		return
	}
	if r.Action == core.ActionBlocked {
		fmt.Fprintln(os.Stderr, "blocked:", label+":", r.Error)
	} else {
		fmt.Fprintln(os.Stderr, "error:", label+":", r.Error)
	}
}

// Bury emits a receipt for every argument before returning a partial failure.
func ReportBury(results []core.GraveResult, asJSON bool, describe func(core.GraveResult) string) error {
	if asJSON {
		if err := PrintJSON(results); err != nil {
			return err
		}
	} else {
		for _, r := range results {
			printBuryResult(r, describe)
		}
	}
	if core.GraveFailed(results) {
		return ErrReported
	}
	return nil
}

func printBuryResult(r core.GraveResult, describe func(core.GraveResult) string) {
	if r.Warning != "" {
		fmt.Fprintln(os.Stderr, "warning:", r.Warning)
	}
	if r.Error == "" {
		fmt.Println(describe(r))
		return
	}
	detail := r.Error
	if r.Detail != "" {
		detail = r.Detail
	}
	fmt.Printf("%s  %s: %s\n", r.Action, r.Ref, detail)
}

// errReported marks failures whose details were already printed.
var ErrReported = errors.New("one or more items failed")

const TimeLayout = "2006-01-02 15:04"

// committed turns an audit-log failure into a warning: the change itself was saved.
func Committed(err error) error {
	if OnlyAuditErrors(err) {
		fmt.Fprintln(os.Stderr, "warning:", err)
		return nil
	}
	return err
}

func PrintJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func When(t time.Time) string {
	return fmt.Sprintf("%s (%s)", t.Local().Format(TimeLayout), textutil.DueIn(time.Until(t)))
}

func Report(results []core.Result, asJSON bool) error {
	if asJSON {
		if err := PrintJSON(results); err != nil {
			return err
		}
	} else {
		for _, r := range results {
			if r.Error != "" {
				fmt.Fprintln(os.Stderr, "error:", r.Error)
				continue
			}
			fmt.Printf("#%d %-8s %s  expires %s\n", r.Item.ID, r.Action, r.Item.Path, When(r.Item.ExpiresAt))
		}
	}
	if core.Failed(results) {
		return ErrReported
	}
	return nil
}

func AnyOK(results []core.Result) bool {
	for _, r := range results {
		if r.Error == "" {
			return true
		}
	}
	return false
}

func IsTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
