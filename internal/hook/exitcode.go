package hook

// EncodeExitCodeResponse is the private transport consumed by the managed
// Windows PowerShell wrapper. Only synchronous PreToolUse denial is bridged;
// obit hook itself still exits zero. Hosts use their native encoder for context.
func EncodeExitCodeResponse(ev Event, result Result) ([]byte, error) {
	if ev.Name != "PreToolUse" || !result.Deny || result.Reason == "" {
		return []byte("{}\n"), nil
	}
	return EncodeJSON(struct {
		Version int    `json:"version"`
		Event   string `json:"event"`
		Deny    bool   `json:"deny"`
		Reason  string `json:"reason"`
	}{1, "PreToolUse", true, capContext(result.Reason)})
}
