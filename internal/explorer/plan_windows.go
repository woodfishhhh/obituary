package explorer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"github.com/woodfishhhh/obituary/internal/winfs"
	"golang.org/x/sys/windows"
)

const iniName = "desktop.ini"

type snapshot struct {
	info  os.FileInfo
	data  []byte
	attrs uint32
}

// Plan holds the exact original snapshot. Apply refuses stale content or identity.
type Plan struct {
	authorized  authorization
	dir         string
	directory   os.FileInfo
	dirAttrs    uint32
	before      snapshot
	after       []byte
	remove      bool
	fileAttrs   uint32
	folderAttrs uint32
	names       []string
	streams     streamSnapshot
	security    *windows.SECURITY_DESCRIPTOR
}

// Prepare plans a complete set of dates for this directory. Existing user keys
// win; reserved infrastructure names are skipped, other unrepresentable names
// are refused. Authorization binds scope/Git discovery and ledger children to
// the same directory identity, captured while that directory was pinned.
func Prepare(dir string, authorized authorization, dates map[string]time.Time) (*Plan, error) {
	expected := authorized.Identity
	abs, err := pathutil.Abs(dir)
	if err != nil {
		return nil, err
	}
	handle, err := pinDirectory(abs)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(handle)
	identity, err := winfs.IdentifyHandle(handle, abs)
	if err != nil {
		return nil, err
	}
	if identity != expected {
		return nil, errors.New("explorer directory changed before planning")
	}
	if err := authorized.verify(handle, abs); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	p, err := preparePinned(abs, identity, root, dates)
	if err == nil {
		p.authorized = authorized
	}
	return p, err
}

func preparePinned(abs string, identity core.Identity, root *os.Root, dates map[string]time.Time) (*Plan, error) {
	info, err := root.Stat(".")
	if err != nil {
		return nil, err
	}
	before, err := readSnapshot(root)
	if err != nil {
		return nil, err
	}
	attrs, err := attributes(abs)
	if err != nil {
		return nil, err
	}
	text, err := decode(before.data)
	if err != nil {
		return nil, err
	}
	base, owned, err := unmerge(text)
	if err != nil {
		return nil, err
	}
	if owned == nil {
		owned = &receipt{Directory: identity, Original: before.data, Existed: before.info != nil, FileAttrs: before.attrs, DirAttrs: attrs}
	}
	if owned.Directory != identity {
		return nil, errors.New("explorer receipt belongs to another directory")
	}
	labels, err := displayNames(root, dates)
	if err != nil {
		return nil, err
	}
	after, remove, err := merge(base, labels, *owned)
	if err != nil {
		return nil, err
	}
	p := &Plan{dir: abs, directory: info, dirAttrs: attrs, before: before, after: after, remove: remove, names: changedNames(labels, owned.Lines)}
	// A newly created INI initially has no user attributes. Archive/Normal
	// are supplied by file creation/rename; Hidden/System belong to labels.
	allowed := owned.FileAttrs | windows.FILE_ATTRIBUTE_HIDDEN | windows.FILE_ATTRIBUTE_SYSTEM | windows.FILE_ATTRIBUTE_ARCHIVE | windows.FILE_ATTRIBUTE_NORMAL
	if p.remove && before.attrs&^allowed != 0 {
		p.remove = false
	}
	p.planAttributes(*owned)
	return p, nil
}

func changedNames(labels map[string]string, previous string) []string {
	names := make(map[string]string)
	for _, line := range strings.Split(previous, "\r\n") {
		name, value, ok := strings.Cut(line, "=")
		if ok && filepath.IsLocal(name) && filepath.Base(name) == name {
			names[name] = value
		}
	}
	result := make([]string, 0, len(names))
	for name, value := range labels {
		if names[name] != value {
			result = append(result, name)
		}
		delete(names, name)
	}
	for name := range names {
		result = append(result, name)
	}
	return result
}

func (p *Plan) planAttributes(owned receipt) {
	p.fileAttrs, p.folderAttrs = p.before.attrs|windows.FILE_ATTRIBUTE_HIDDEN|windows.FILE_ATTRIBUTE_SYSTEM, p.dirAttrs|windows.FILE_ATTRIBUTE_READONLY
	if !bytes.Contains(p.after, encode(marker)[2:]) {
		p.fileAttrs = restoreBits(p.before.attrs, owned.FileAttrs, windows.FILE_ATTRIBUTE_HIDDEN|windows.FILE_ATTRIBUTE_SYSTEM)
		p.folderAttrs = restoreBits(p.dirAttrs, owned.DirAttrs, windows.FILE_ATTRIBUTE_READONLY)
	}
}

func displayNames(root *os.Root, dates map[string]time.Time) (map[string]string, error) {
	labels := make(map[string]string, len(dates))
	for name, date := range dates {
		if pathutil.IsExplorerMetadata(name) {
			continue
		}
		if !filepath.IsLocal(name) || filepath.Base(name) != name || strings.ContainsAny(name, "=\r\n\x00[]\";") || strings.TrimSpace(name) != name {
			return nil, fmt.Errorf("unrepresentable Explorer label name %q", name)
		}
		info, err := root.Lstat(name)
		if err != nil {
			return nil, err
		}
		stem := name
		if !info.IsDir() {
			if base := strings.TrimSuffix(name, filepath.Ext(name)); base != "" {
				stem = base
			}
		}
		labels[name] = stem + "（" + date.In(time.Local).Format("01-02") + " 下葬）"
	}
	return labels, nil
}

func readSnapshot(root *os.Root) (snapshot, error) {
	info, err := root.Lstat(iniName)
	if errors.Is(err, os.ErrNotExist) {
		return snapshot{}, nil
	}
	if err != nil {
		return snapshot{}, err
	}
	attrs := info.Sys().(*syscall.Win32FileAttributeData).FileAttributes
	if !info.Mode().IsRegular() || attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return snapshot{}, errors.New("desktop.ini is not an ordinary file")
	}
	f, err := root.Open(iniName)
	if err != nil {
		return snapshot{}, err
	}
	defer f.Close()
	return snapshotFile(f, info)
}

func snapshotFile(f *os.File, info os.FileInfo) (snapshot, error) {
	var stat windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &stat); err != nil {
		return snapshot{}, err
	}
	if stat.NumberOfLinks != 1 {
		return snapshot{}, errors.New("desktop.ini has multiple hard links")
	}
	if stat.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY) != 0 {
		return snapshot{}, errors.New("desktop.ini is not an ordinary file")
	}
	opened, err := f.Stat()
	if err != nil {
		return snapshot{}, err
	}
	if !os.SameFile(info, opened) {
		return snapshot{}, errors.New("desktop.ini changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxINIBytes+1))
	if len(data) > maxINIBytes {
		return snapshot{}, errors.New("desktop.ini exceeds 1 MiB")
	}
	return snapshot{info: opened, data: data, attrs: stat.FileAttributes}, err
}

func restoreBits(current, original, mask uint32) uint32 { return current&^mask | original&mask }
