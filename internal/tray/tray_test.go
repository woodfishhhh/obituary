//go:build windows

package tray

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"os"
	"testing"
)

func TestDataDirectoryLauncherExistsAndKeepsOneOperand(t *testing.T) {
	path := `C:\data & archives\讣告`
	cmd, err := directoryCommand(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cmd.Path); err != nil {
		t.Fatalf("installed Explorer launcher missing: %v", err)
	}
	if len(cmd.Args) != 2 || cmd.Args[1] != path {
		t.Fatalf("directory operand changed: %q", cmd.Args)
	}
}

func TestEmbeddedIconHasWindowsTraySizes(t *testing.T) {
	if len(icon) < 6 || binary.LittleEndian.Uint16(icon[2:]) != 1 {
		t.Fatal("not an ICO")
	}
	count := int(binary.LittleEndian.Uint16(icon[4:]))
	if len(icon) < 6+count*16 {
		t.Fatal("incomplete icon directory")
	}
	sizes := map[byte]bool{}
	for i := 0; i < count; i++ {
		entry := icon[6+i*16:]
		sizes[entry[0]] = true
		length, offset := binary.LittleEndian.Uint32(entry[8:]), binary.LittleEndian.Uint32(entry[12:])
		if uint64(offset)+uint64(length) > uint64(len(icon)) {
			t.Fatal("truncated ICO payload")
		}
		frame, err := png.DecodeConfig(bytes.NewReader(icon[offset : offset+length]))
		if err != nil {
			t.Fatal(err)
		}
		size := int(entry[0])
		if size == 0 {
			size = 256
		}
		if frame.Width != size || frame.Height != size {
			t.Fatalf("ICO layer %d has payload %dx%d", size, frame.Width, frame.Height)
		}
	}
	// ICO represents a 256px dimension with zero.
	for _, size := range []byte{16, 20, 24, 32, 40, 48, 64, 128, 0} {
		if !sizes[size] {
			t.Fatalf("missing size %d", size)
		}
	}
}
