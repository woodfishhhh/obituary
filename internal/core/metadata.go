package core

// ReadMetadata reads small application-owned metadata while permitting atomic
// replacement by another process on Windows.
func ReadMetadata(path string) ([]byte, error) { return readShared(path) }

// WriteMetadata publishes application-owned metadata with the store's atomic
// replacement protocol. The caller must create the parent and serialize writers.
func WriteMetadata(path string, data []byte) error { return writeDurably(path, data) }
