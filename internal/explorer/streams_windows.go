package explorer

import (
	"crypto/sha256"
	"errors"
	"io"
	"maps"
	"os"

	"github.com/Microsoft/go-winio"
)

type streamKey struct {
	ID     uint32
	Name   string
	Offset int64
}

type streamState struct {
	Size       int64
	Attributes uint32
	Hash       [sha256.Size]byte
}

type streamSnapshot map[streamKey]streamState

// go-winio/backup.go already implements the BackupRead stream framing and
// sparse headers. Hash exactly the bytes copied, retaining size and stream
// names so both additions/removals and same-sized edits fail the final check.
// CopyFileW/os.File.Stat cannot supply a snapshot of named streams or EAs.
func backupStreams(source, target *os.File) (streamSnapshot, error) {
	reader := winio.NewBackupFileReader(source, false)
	defer reader.Close()
	input := winio.NewBackupStreamReader(reader)
	var output *winio.BackupStreamWriter
	if target != nil {
		writer := winio.NewBackupFileWriter(target, false)
		defer writer.Close()
		output = winio.NewBackupStreamWriter(writer)
	}
	result := make(streamSnapshot)
	for {
		header, err := input.Next()
		if errors.Is(err, io.EOF) {
			return result, nil
		}
		if err != nil {
			return nil, err
		}
		state, err := transferStream(input, output, header)
		if err != nil {
			return nil, err
		}
		key := streamKey{header.Id, header.Name, header.Offset}
		if _, exists := result[key]; exists {
			return nil, errors.New("desktop.ini has ambiguous backup streams")
		}
		result[key] = state
	}
}

func transferStream(input io.Reader, output *winio.BackupStreamWriter, header *winio.BackupHeader) (streamState, error) {
	hash := sha256.New()
	var sink io.Writer = hash
	if output != nil {
		if err := output.WriteHeader(header); err != nil {
			return streamState{}, err
		}
		sink = io.MultiWriter(output, hash)
	}
	size, err := io.Copy(sink, input)
	if err == nil && size != header.Size {
		err = io.ErrUnexpectedEOF
	}
	return streamState{Size: size, Attributes: header.Attributes, Hash: [sha256.Size]byte(hash.Sum(nil))}, err
}

func sameStreams(a, b streamSnapshot) bool { return maps.Equal(a, b) }
