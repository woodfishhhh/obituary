package cmdutil

import (
	"bytes"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

func TestQuotaReminderFormatting(t *testing.T) {
	overflow := &lifecycle.QuotaOverflow{
		ProposalID: 42,
		Path:       `C:\tmp\huge.bin`,
		ItemBytes:  8 << 30,
		MaxBytes:   5 << 30,
	}
	en := i18n.NewLocalizer(i18n.English)
	msgEN := FormatQuotaOverflow(en, overflow, false)
	for _, want := range []string{`C:\tmp\huge.bin`, "8.0 GiB", "5.0 GiB", "#42", "raw rm"} {
		if !strings.Contains(msgEN, want) {
			t.Fatalf("EN overflow message missing %q: %s", want, msgEN)
		}
	}
	zh := i18n.NewLocalizer(i18n.Chinese)
	msgZH := FormatQuotaOverflow(zh, overflow, false)
	for _, want := range []string{`C:\tmp\huge.bin`, "8.0 GiB", "5.0 GiB", "待你决定", "rm"} {
		if !strings.Contains(msgZH, want) {
			t.Fatalf("ZH overflow message missing %q: %s", want, msgZH)
		}
	}

	ev := &lifecycle.QuotaEviction{
		MaxBytes:    5 << 30,
		BeforeBytes: 6 << 30,
		AfterBytes:  4 << 30,
		FreedBytes:  2 << 30,
		PurgedCount: 2,
		FailedCount: 1,
	}
	var buf bytes.Buffer
	PrintQuotaEviction(&buf, zh, ev, false)
	got := buf.String()
	for _, want := range []string{"5.0 GiB", "2 项", "2.0 GiB", "4.0 GiB / 5.0 GiB", "1 项"} {
		if !strings.Contains(got, want) {
			t.Fatalf("ZH eviction message missing %q: %s", want, got)
		}
	}
}
