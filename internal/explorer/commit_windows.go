package explorer

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/woodfishhhh/obituary/internal/winfs"
	"golang.org/x/sys/windows"
)

func (p *Plan) copyOriginal(old, temporary *os.File) error {
	if old == nil {
		return nil
	}
	if _, err := old.Seek(0, io.SeekStart); err != nil {
		return err
	}
	var err error
	p.security, err = readSecurity(old)
	if err != nil {
		return err
	}
	p.streams, err = backupStreams(old, temporary)
	if err != nil {
		return err
	}
	if err := copySecurity(p.security, temporary); err != nil {
		return err
	}
	if err := temporary.Truncate(0); err != nil {
		return err
	}
	_, err = temporary.Seek(0, io.SeekStart)
	return err
}

func (p *Plan) verifyFinal(old *os.File) error {
	current, err := backupStreams(old, nil)
	if err != nil {
		return err
	}
	if !sameStreams(current, p.streams) {
		return errors.New("desktop.ini streams changed before commit; retry next sync")
	}
	sd, err := readSecurity(old)
	if err != nil {
		return err
	}
	if !bytes.Equal(securityBytes(sd), securityBytes(p.security)) {
		return errors.New("desktop.ini security changed before commit; retry next sync")
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(old.Fd()), &info); err != nil {
		return err
	}
	if info.FileAttributes != p.before.attrs {
		return errors.New("desktop.ini attributes changed before commit; retry next sync")
	}
	return nil
}

func (p *Plan) captureRemoval(old *os.File) error {
	// Capture before the ownership enumeration. A stream added between that
	// enumeration and deletion must differ from this snapshot, not become part
	// of a later baseline that would silently authorize deleting it.
	var err error
	p.streams, err = backupStreams(old, nil)
	if err != nil {
		return err
	}
	p.security, err = readSecurity(old)
	return err
}

func (p *Plan) commitOriginal() (*os.File, error) {
	if p.before.info == nil {
		return nil, nil
	}
	access := uint32(0)
	if p.remove {
		access = windows.DELETE
	}
	f, err := winfs.OpenCommitFile(filepath.Join(p.dir, iniName), access)
	if err != nil {
		return nil, err
	}
	current, err := snapshotFile(f, p.before.info)
	if err == nil && (!bytes.Equal(current.data, p.before.data) || current.attrs != p.before.attrs) {
		err = errors.New("desktop.ini changed before commit")
	}
	if err == nil {
		err = labelsCommitHook()
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// FileStreamInfo is documented by Microsoft and exposed by x/sys/windows.
// os.File.Stat only describes the unnamed stream. A single-entry buffer is
// enough to prove its absence of siblings; errors or longer lists preserve
// the file instead of treating an incomplete enumeration as ownership.
func onlyDefaultStream(f *os.File) bool {
	info := struct {
		Next, NameBytes uint32
		Size, Allocated int64
		Name            [8]uint16
	}{}
	err := windows.GetFileInformationByHandleEx(windows.Handle(f.Fd()), windows.FileStreamInfo,
		(*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	return err == nil && info.Next == 0 && info.NameBytes == 14 && windows.UTF16ToString(info.Name[:]) == "::$DATA"
}
