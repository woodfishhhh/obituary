package explorer

import (
	"golang.org/x/sys/unix"
	"strings"
)

const tagAttr = "user.xdg.tags"
const noTagAttr = unix.ENODATA

func decodeTags(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	return strings.Split(string(raw), ","), nil
}
func encodeTags(tags []string) ([]byte, error) { return []byte(strings.Join(tags, ",")), nil }
