package fs

// FileAncestry holds a regular file and its complete directory ancestry against
// rename, on one verified local filesystem with stable IDs. Unsupported, linked,
// case-sensitive or unverifiable paths must not produce this capability.
type FileAncestry interface {
	Path() string
	Identities() []Identity
	Close() error
}
