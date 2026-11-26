//go:build darwin

package winfs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

type hintEntry struct {
	name       string
	directory  bool
	mountPoint bool
}

const (
	attrDirMountStatus = 0x00000004
	dirMountPoint      = 0x00000001
	vdir               = 2
)

func readHintEntries(dir *os.File, limit int) ([]hintEntry, error) {
	if limit <= 0 {
		return nil, nil
	}
	attrs := unix.Attrlist{
		Bitmapcount: unix.ATTR_BIT_MAP_COUNT,
		Commonattr:  unix.ATTR_CMN_RETURNED_ATTRS | unix.ATTR_CMN_NAME | unix.ATTR_CMN_ERROR | unix.ATTR_CMN_OBJTYPE,
		Dirattr:     attrDirMountStatus,
	}
	buf := make([]byte, limit*2048)
	count, _, errno := unix.Syscall6(unix.SYS_GETATTRLISTBULK,
		uintptr(dir.Fd()), uintptr(unsafe.Pointer(&attrs)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0, 0)
	if errno != 0 {
		return nil, errno
	}
	if count == 0 {
		return nil, io.EOF
	}
	entries := make([]hintEntry, 0, count)
	offset := 0
	for i := 0; i < int(count); i++ {
		if offset+24 > len(buf) {
			return nil, fmt.Errorf("short getattrlistbulk record")
		}
		recordLen := int(binary.LittleEndian.Uint32(buf[offset : offset+4]))
		if recordLen < 24 || offset+recordLen > len(buf) {
			return nil, fmt.Errorf("invalid getattrlistbulk record length %d", recordLen)
		}
		entry, err := parseHintEntry(buf[offset : offset+recordLen])
		if err != nil {
			return nil, err
		}
		if entry.name != "" {
			entries = append(entries, entry)
		}
		offset += recordLen
	}
	return entries, nil
}

func parseHintEntry(record []byte) (hintEntry, error) {
	returnedCommon := binary.LittleEndian.Uint32(record[4:8])
	returnedDir := binary.LittleEndian.Uint32(record[12:16])
	field := 24
	if returnedCommon&unix.ATTR_CMN_ERROR != 0 {
		if field+4 > len(record) {
			return hintEntry{}, fmt.Errorf("short getattrlistbulk error field")
		}
		if binary.LittleEndian.Uint32(record[field:field+4]) != 0 {
			return hintEntry{}, nil
		}
		field += 4
	}
	if returnedCommon&unix.ATTR_CMN_NAME == 0 || field+8 > len(record) {
		return hintEntry{}, fmt.Errorf("getattrlistbulk entry has no name")
	}
	nameOffset := int(int32(binary.LittleEndian.Uint32(record[field : field+4])))
	nameLength := int(binary.LittleEndian.Uint32(record[field+4 : field+8]))
	nameStart := field + nameOffset
	if nameOffset < 0 || nameLength <= 0 || nameStart < 0 || nameStart+nameLength > len(record) {
		return hintEntry{}, fmt.Errorf("invalid getattrlistbulk name reference")
	}
	nameBytes := record[nameStart : nameStart+nameLength]
	if end := bytes.IndexByte(nameBytes, 0); end >= 0 {
		nameBytes = nameBytes[:end]
	}
	field += 8
	objType := uint32(0)
	if returnedCommon&unix.ATTR_CMN_OBJTYPE != 0 {
		if field+4 > len(record) {
			return hintEntry{}, fmt.Errorf("short getattrlistbulk object type")
		}
		objType = binary.LittleEndian.Uint32(record[field : field+4])
		field += 4
	}
	mountStatus := uint32(0)
	if returnedDir&attrDirMountStatus != 0 {
		if field+4 > len(record) {
			return hintEntry{}, fmt.Errorf("short getattrlistbulk mount status")
		}
		mountStatus = binary.LittleEndian.Uint32(record[field : field+4])
	}
	return hintEntry{
		name:       string(nameBytes),
		directory:  objType == vdir,
		mountPoint: mountStatus&dirMountPoint != 0,
	}, nil
}
