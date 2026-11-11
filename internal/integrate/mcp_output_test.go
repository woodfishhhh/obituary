package integrate

import (
	"bytes"
	"fmt"
	"testing"
)

func TestClaudeOutputRetainsPrefixAndSuffix(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, 1, claudeOutputLimit, 2 * claudeOutputLimit, 2*claudeOutputLimit + 1, 9 * claudeOutputLimit} {
		for _, chunk := range []int{7, claudeOutputLimit + 3, 10 * claudeOutputLimit} {
			data := make([]byte, size)
			for i := range data {
				data[i] = byte(i % 251)
			}
			var w claudeOutput
			for start := 0; start < size; start += chunk {
				p := data[start:min(start+chunk, size)]
				if n, err := w.Write(p); err != nil || n != len(p) {
					t.Fatalf("output not drained: %d %v", n, err)
				}
			}
			want := data
			if size > 2*claudeOutputLimit {
				want = append(bytes.Clone(data[:claudeOutputLimit]), []byte(fmt.Sprintf("\n... omitting %d bytes ...\n", size-2*claudeOutputLimit))...)
				want = append(want, data[size-claudeOutputLimit:]...)
			}
			// Reading the buffered output twice must not consume it.
			got := w.Bytes()
			if !bytes.Equal(got, want) || !bytes.Equal(w.Bytes(), got) {
				t.Fatalf("output changed for size=%d chunk=%d", size, chunk)
			}
		}
	}
}
