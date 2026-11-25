package explorer

import (
	"golang.org/x/sys/unix"
	"howett.net/plist"
)

const tagAttr = "com.apple.metadata:_kMDItemUserTags"
const noTagAttr = unix.ENOATTR

func decodeTags(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var tags []string
	_, err := plist.Unmarshal(raw, &tags)
	return tags, err
}
func encodeTags(tags []string) ([]byte, error) { return plist.Marshal(tags, plist.BinaryFormat) }
