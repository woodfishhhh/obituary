package integrate

import (
	"fmt"
	"strconv"

	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

// PlanLanguageConfig reuses PlanGraveyardConfig's go-toml AST edits and the
// original-check/backup/atomic publication provided by Apply.
func PlanLanguageConfig(path, language string) (Change, error) {
	if !i18n.Valid(language) {
		return Change{}, fmt.Errorf("ui.language must be %q, %q or %q", i18n.Auto, i18n.Chinese, i18n.English)
	}
	return planConfigValue(path, "ui", "language", strconv.Quote(language))
}

// PlanSessionSummaryConfig sets [hooks] session_summary the same way.
func PlanSessionSummaryConfig(path string, enabled bool) (Change, error) {
	return planConfigValue(path, "hooks", "session_summary", strconv.FormatBool(enabled))
}

// PlanSessionCleanupConfig sets [hooks] session_cleanup the same way.
func PlanSessionCleanupConfig(path string, enabled bool) (Change, error) {
	return planConfigValue(path, "hooks", "session_cleanup", strconv.FormatBool(enabled))
}

// PlanProposalModeConfig sets [proposals] mode the same way.
func PlanProposalModeConfig(path, mode string) (Change, error) {
	switch mode {
	case config.ProposalsOff, config.ProposalsTiered, config.ProposalsAuto, config.ProposalsAsk:
	default:
		return Change{}, fmt.Errorf("proposals.mode must be %q, %q, %q or %q", config.ProposalsOff, config.ProposalsTiered, config.ProposalsAuto, config.ProposalsAsk)
	}
	return planConfigValue(path, "proposals", "mode", strconv.Quote(mode))
}

// planConfigValue edits one key of the global config and checks that the file
// still parses before and after the edit.
func planConfigValue(path, table, key, literal string) (Change, error) {
	ch, err := readIntegrationChange(Scope{Global: true}, path)
	if err != nil {
		return ch, err
	}
	if _, err := config.Parse(ch.orig, path); err != nil {
		return ch, err
	}
	data := ch.orig
	if !ch.existed {
		data = []byte(config.Template)
	}
	data, err = setConfigValue(data, table, key, literal)
	if err != nil {
		return ch, err
	}
	if _, err := config.Parse(data, path); err != nil {
		return ch, err
	}
	ch.Detail = fmt.Sprintf("%s.%s=%s", table, key, literal)
	return setIntegrationData(ch, data), nil
}
