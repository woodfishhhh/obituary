//go:build linux || darwin

package winfs

import (
	"encoding/hex"
	"errors"
	"fmt"
	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"path/filepath"
)

// A purge owns only the recorded payload and its private staging directory.
func (f FS) PurgeGrave(path string, want corefs.Identity, plan corefs.PurgePlan) error {
	parent, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer parent.Close()
	name := filepath.Base(path)
	root, err := openOnMount(int(parent.Fd()), name, unix.O_RDONLY|unix.O_DIRECTORY)
	if err != nil {
		return err
	}
	defer unix.Close(root)
	st, err := f.statAt(root, "")
	if err != nil {
		return err
	}
	if st.Dev != want.Serial || purgeID(st) != want.FileID || !plan.ValidStage() {
		return corefs.ErrUnsafe
	}
	tree := purgeTree{fs: f, root: root, stage: -1, plan: plan}
	if err := tree.purgePayload(); err != nil {
		return err
	}
	return plan.WithObject(want.FileID, st.Incarnation == 0, func() error { return unix.Unlinkat(int(parent.Fd()), name, unix.AT_REMOVEDIR) })
}

type purgeTree struct {
	fs          FS
	root, stage int
	plan        corefs.PurgePlan
}

func purgeID(st posixStat) string { id := inodeBytes(&st); return hex.EncodeToString(id[:]) }

func (p *purgeTree) purgePayload() (err error) {
	name := p.plan.Name
	st, err := p.fs.burialStat(p.root, name)
	if err != nil {
		return err
	}
	if st.Dev != p.plan.Object.Serial || purgeID(st) != p.plan.Object.FileID {
		return corefs.ErrUnsafe
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR {
		return p.entry(p.root, name, st)
	}
	if err = p.openStage(); err != nil {
		return err
	}
	defer unix.Close(p.stage)
	stageStat, statErr := p.fs.statAt(p.stage, "")
	if statErr != nil {
		return statErr
	}
	if stageStat.Dev != p.plan.Object.Serial || stageStat.Mode&0077 != 0 {
		return corefs.ErrUnsafe
	}
	if err := p.plan.WithObject(purgeID(stageStat), stageStat.Incarnation == 0, nil); err != nil {
		return err
	}
	defer func() {
		if err == nil {
			return
		}
		e := p.unlink(p.root, p.plan.Stage, stageStat)
		if e != nil && !errors.Is(e, unix.ENOTEMPTY) && !errors.Is(e, unix.ENOENT) {
			err = errors.Join(err, e)
		}
	}()
	if err = p.resumeStage(); err != nil {
		return err
	}
	return p.entry(p.root, name, st)
}

func (p *purgeTree) openStage() error {
	created := true
	err := unix.Mkdirat(p.root, p.plan.Stage, 0700)
	if errors.Is(err, unix.EEXIST) {
		created = false
	} else if err != nil {
		return err
	}
	p.stage, err = openOnMount(p.root, p.plan.Stage, unix.O_RDONLY|unix.O_DIRECTORY)
	if err != nil && created {
		return errors.Join(err, unix.Unlinkat(p.root, p.plan.Stage, unix.AT_REMOVEDIR))
	}
	return err
}

// Each staged name is its expected file identity. Only emptied directories are staged.
func (p *purgeTree) resumeStage() error {
	return p.each(p.stage, func(fd int, name string) error {
		st, err := p.fs.burialStat(fd, name)
		if err != nil {
			return err
		}
		if st.Dev != p.plan.Object.Serial || purgeID(st) != name && !p.plan.LegacyMembers[purgeID(st)] {
			return corefs.ErrUnsafe
		}
		return p.unlink(fd, name, st)
	})
}

func (p *purgeTree) each(fd int, visit func(int, string) error) error {
	dup, err := unix.Dup(fd)
	if err != nil {
		return err
	}
	dir := os.NewFile(uintptr(dup), "grave")
	defer dir.Close()
	for {
		names, err := dir.Readdirnames(64)
		for _, name := range names {
			if err := visit(fd, name); err != nil {
				return err
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (p *purgeTree) entry(parent int, name string, st posixStat) error {
	if st.Dev != p.plan.Object.Serial {
		return corefs.ErrUnsafe
	}
	if st.Mode&unix.S_IFMT == unix.S_IFDIR {
		if err := p.emptyDirectory(parent, name, st); err != nil {
			return err
		}
	}

	if parent == p.root {
		if err := p.finishStage(); err != nil {
			return err
		}
		return p.unlink(parent, name, st)
	}

	id := purgeID(st)
	if err := renameExclusive(parent, name, p.stage, id); err != nil {
		return err
	}
	if err := p.unlink(p.stage, id, st); err != nil {
		return errors.Join(err, renameExclusive(p.stage, id, parent, name))
	}
	return nil
}

// unlinkat accepts names; Linux compares on a pinned handle, while Darwin uses no-follow stat for special entries.
func (p *purgeTree) unlink(fd int, name string, want posixStat) error {
	if handled, err := p.tryUnlinkWithoutOpen(fd, name, want, nil); handled {
		return err
	}
	pinned, err := openOnMount(fd, name, moveOpenFlags)
	if err != nil {
		if handled, fallbackErr := p.tryUnlinkWithoutOpen(fd, name, want, err); handled {
			return fallbackErr
		}
		return err
	}
	defer unix.Close(pinned)
	got, err := p.fs.statAt(pinned, "")
	if err != nil {
		return err
	}
	if got.Dev != want.Dev || inodeBytes(&got) != inodeBytes(&want) {
		return fmt.Errorf("changed entry %s: %w", name, corefs.ErrUnsafe)
	}
	flags := 0
	if got.Mode&unix.S_IFMT == unix.S_IFDIR {
		flags = unix.AT_REMOVEDIR
	}
	return p.plan.WithObject(purgeID(got), got.Incarnation == 0, func() error { return unix.Unlinkat(fd, name, flags) })
}

func (p *purgeTree) unlinkByName(fd int, name string, want posixStat) error {
	got, err := p.fs.statAt(fd, name)
	if err != nil {
		return err
	}
	if got.Dev != want.Dev || inodeBytes(&got) != inodeBytes(&want) || got.Mode&unix.S_IFMT == unix.S_IFDIR {
		return fmt.Errorf("changed entry %s: %w", name, corefs.ErrUnsafe)
	}
	return p.plan.WithObject(purgeID(got), got.Incarnation == 0, func() error { return unix.Unlinkat(fd, name, 0) })
}

func (p *purgeTree) emptyDirectory(parent int, name string, st posixStat) error {
	fd, err := openOnMount(parent, name, unix.O_RDONLY|unix.O_DIRECTORY)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	got, err := p.fs.statAt(fd, "")
	if err != nil {
		return err
	}
	if got.Dev != st.Dev || inodeBytes(&got) != inodeBytes(&st) {
		return corefs.ErrUnsafe
	}
	if err = p.plan.WithObject(purgeID(got), got.Incarnation == 0, nil); err != nil {
		return err
	}
	if err = p.each(fd, func(fd int, child string) error {
		cs, e := p.fs.burialStat(fd, child)
		if e != nil {
			return e
		}
		return p.entry(fd, child, cs)
	}); err != nil {
		return err
	}
	return nil
}

func (p *purgeTree) finishStage() error {
	if p.stage < 0 {
		return nil
	}
	st, err := p.fs.statAt(p.stage, "")
	if err != nil {
		return err
	}
	return p.unlink(p.root, p.plan.Stage, st)
}
