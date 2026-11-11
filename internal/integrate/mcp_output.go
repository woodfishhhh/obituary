package integrate

import (
	"bytes"
	"fmt"
)

// claudeOutput keeps the first and last 32 KiB, consumes everything, and reports
// the omitted byte count to bound memory while retaining useful diagnostics.
// A shared writer for Stdout/Stderr is serialized by os/exec.
type claudeOutput struct {
	prefix, suffix []byte
	offset         int
	skipped        int64
}

const claudeOutputLimit = 32 << 10

func (w *claudeOutput) Write(p []byte) (int, error) {
	n := len(p)
	fill := func(dst *[]byte) {
		count := min(claudeOutputLimit-len(*dst), len(p))
		*dst = append(*dst, p[:count]...)
		p = p[count:]
	}
	fill(&w.prefix)
	if excess := len(p) - claudeOutputLimit; excess > 0 {
		w.skipped += int64(excess)
		p = p[excess:]
	}
	fill(&w.suffix)
	for len(p) > 0 {
		count := copy(w.suffix[w.offset:], p)
		p = p[count:]
		w.skipped += int64(count)
		w.offset = (w.offset + count) % claudeOutputLimit
	}
	return n, nil
}

func (w *claudeOutput) Bytes() []byte {
	var out bytes.Buffer
	out.Grow(len(w.prefix) + len(w.suffix) + 64)
	out.Write(w.prefix)
	if w.skipped > 0 {
		fmt.Fprintf(&out, "\n... omitting %d bytes ...\n", w.skipped)
	}
	out.Write(w.suffix[w.offset:])
	out.Write(w.suffix[:w.offset])
	return out.Bytes()
}
