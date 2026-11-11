package ledger

// LeftoverRejected also excludes collapsed directories containing a rejected
// path and children of rejected directories, independent of Git's spelling.
func (st *State) LeftoverRejected(project, session, path string) bool {
	for _, r := range st.LeftoverRejections {
		if !samePath(r.Project, project) || r.Session != session {
			continue
		}
		for _, p := range r.Paths {
			if samePath(path, p) || isInside(path, p) || isInside(p, path) {
				return true
			}
		}
	}
	return false
}
