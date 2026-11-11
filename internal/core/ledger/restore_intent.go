package ledger

// Write restore metadata before moving the object and retain the manifest
// until its ledger commit succeeds, so an interrupted restore can be recovered.
type RestoreIntent struct {
	Parent Identity `json:"parent"`
	Name   string   `json:"name"`
}
