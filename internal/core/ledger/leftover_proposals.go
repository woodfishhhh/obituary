package ledger

// LeftoverPathRejected applies a file decision across sessions in its project.
// Reusing the bounded rejection history also covers decisions from old ledgers.
// Container decisions continue to use their session-scoped identity keys.
func (st *State) LeftoverPathRejected(project, path string) bool {
	for _, r := range st.LeftoverRejections {
		if !samePath(r.Project, project) {
			continue
		}
		for _, rejected := range r.Paths {
			if samePath(path, rejected) || isInside(path, rejected) || isInside(rejected, path) {
				return true
			}
		}
	}
	return false
}
