package explorer

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// Successful decoding must survive a round trip to preserve accepted text;
// unsupported encodings may fail.
func FuzzINIMerge(f *testing.F) {
	for _, seed := range iniFuzzSeeds() {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, original []byte) {
		text, err := decode(original)
		if err != nil {
			return
		}
		roundTrip, err := decode(encode(text))
		if err != nil || roundTrip != text {
			t.Fatalf("encoding changed accepted text: %v", err)
		}
		base, owned, err := unmerge(text)
		if err != nil {
			return
		}
		if owned == nil {
			owned = &receipt{Original: bytes.Clone(original), Existed: true}
		}
		labels := map[string]string{"report.log": "report（11-13 下葬）", "other.txt": "other（11-14 下葬）"}
		out, _, err := merge(base, labels, *owned)
		if err != nil {
			return // oversized receipts and invalid original encodings are refused
		}
		merged, err := decode(out)
		if err != nil {
			t.Fatalf("merge produced invalid encoding: %v", err)
		}
		restored, next, err := unmerge(merged)
		if err != nil || restored != base {
			t.Fatalf("merge changed user sections, keys or comments: %v", err)
		}
		if next == nil {
			return // every requested name is already user-owned
		}
		again, _, err := merge(restored, labels, *next)
		if err != nil || !bytes.Equal(again, out) {
			t.Fatalf("same labels are not idempotent: %v", err)
		}
		cleaned, _, err := merge(restored, nil, *next)
		if err != nil {
			t.Fatal(err)
		}
		if owned.Existed && bytes.Equal(owned.Original, original) && !bytes.Equal(cleaned, original) {
			t.Fatal("removing labels did not restore every original byte")
		}
	})
}

func iniFuzzSeeds() [][]byte {
	seeds := [][]byte{{}, {0}, {0xff, 0xfe, 0}, {0xff, 0xfe, 0, 0xd8}, {0xfe, 0xff, 0, 10}}
	for _, text := range []string{
		"; user comment\r\n[.ShellClassInfo]\r\nIconResource=user.ico,0\r\n",
		"[LocalizedFileNames]\nreport.log=User name\n[Other]\nx=1\n[LocalizedFileNames]\nREPORT.LOG=Last name",
		"[LocalizedFileNames]\rreport.log=User name\r",
		"[LocalizedFileNames]", "; " + strings.Repeat("long", 20000),
		"[Other]\n; 注释🙂\nx=\x00\n", marker, endMarker,
	} {
		le := encode(text)
		be := bytes.Clone(le)
		for i := 0; i < len(be); i += 2 {
			binary.BigEndian.PutUint16(be[i:], binary.LittleEndian.Uint16(le[i:]))
		}
		seeds = append(seeds, []byte(text), append([]byte{0xef, 0xbb, 0xbf}, text...), le, le[2:], be, be[2:])
	}
	return seeds
}

func TestINIEncodingIsLosslessOrRefused(t *testing.T) {
	for _, text := range []string{"", "; 注释\r\n[A]\r\nkey=🙂\r\n", "[LocalizedFileNames]"} {
		data := encode(text)
		decoded, err := decode(data)
		if err != nil || decoded != text {
			t.Fatalf("round trip %q: %q %v", text, decoded, err)
		}
	}
	for _, data := range [][]byte{{0xff, 0xfe, 0}, {0xff, 0xfe, 0, 0xd8}, {0xff, 0xfe, 0, 0}, {0xff, 0}, {0xfe, 0xff, 0, 10}, {0x81, 0x82}} {
		if _, err := decode(data); err == nil {
			t.Fatalf("accepted corrupt/unknown encoding: %x", data)
		}
	}
}

func TestINIReceiptNeverExceedsReadLimit(t *testing.T) {
	base := strings.Repeat("; padding\n", 30000)
	if _, _, err := merge(base, map[string]string{"a": "a（11-13 下葬）"}, receipt{Original: []byte(base), Existed: true}); err == nil {
		t.Fatal("wrote an ownership receipt that cannot be read back")
	}
}

func TestINIBlockPreservesMissingFinalNewline(t *testing.T) {
	for _, original := range []string{"[LocalizedFileNames]", "; original", "[A]\r\nx=1\r\n"} {
		r := receipt{Original: []byte(original), Existed: true}
		data, _, err := merge(original, map[string]string{"a.txt": "a（11-13 下葬）"}, r)
		if err != nil {
			t.Fatal(err)
		}
		text, _ := decode(data)
		base, owned, err := unmerge(text)
		if err != nil || base != original {
			t.Fatalf("%q: %q %v", original, base, err)
		}
		data, remove, err := merge(base, nil, *owned)
		if err != nil || remove || !bytes.Equal(data, []byte(original)) {
			t.Fatalf("restore: %q %v %v", data, remove, err)
		}
	}
}

func TestINICleanupKeepsUserSection(t *testing.T) {
	for _, original := range []string{"", "[.ShellClassInfo]\nIconResource=own.ico,0\n"} {
		for _, addition := range []string{"keep.txt=User display name\r\n", "; user comment\r\n"} {
			r := receipt{Original: []byte(original), Existed: original != ""}
			data, _, err := merge(original, map[string]string{"report.log": "report"}, r)
			if err != nil {
				t.Fatal(err)
			}
			text, _ := decode(data)
			base, owned, err := unmerge(text + addition)
			if err != nil {
				t.Fatal(err)
			}
			data, remove, err := merge(base, nil, *owned)
			text, _ = decode(data)
			if err != nil || remove || text != original+"[LocalizedFileNames]\r\n"+addition {
				t.Fatalf("cleanup lost user section: %q remove=%v err=%v", text, remove, err)
			}
		}
	}
}
