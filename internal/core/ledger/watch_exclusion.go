package ledger

// WatchExclusion keeps a removed nested folder outside its ancestor's policy.
// ID is the removed policy ID, so it remains addressable after a path disappears.
type WatchExclusion struct {
	ID       int64    `json:"id"`
	Path     string   `json:"path"`
	Identity Identity `json:"identity"`
}
