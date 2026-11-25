//go:build linux || darwin

package explorer

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/winfs"
	"golang.org/x/sys/unix"
)

func projectLabels(svc *core.Service, index string, state core.State, receipts map[string]directory, options Options) error {
	if receipts == nil {
		receipts = map[string]directory{}
	}
	wanted := map[string]string{}
	if options.Enabled {
		if err := collectWanted(svc, state, receipts, options, wanted); err != nil {
			return err
		}
	}
	var failures []error
	for key, r := range receipts {
		path := r.Path
		if wanted[key] == "" {
			path = relocated(state, r)
		}
		err := updateTags(path, r, wanted[key], func(old, next string) error {
			if next == "" && old == "" {
				delete(receipts, key)
			} else {
				r.Candidate = false
				r.Tag = next
				r.PreviousTag = old
				r.Path = path
				receipts[key] = r
			}
			return saveDirectories(index, receipts)
		})
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", path, err))
		}
	}
	return errors.Join(failures...)
}

// collectWanted records, for every tracked item that should carry a date tag, the tag it wants.
func collectWanted(svc *core.Service, state core.State, receipts map[string]directory, options Options, wanted map[string]string) error {
	for _, it := range state.Items {
		if it.Identity == nil {
			continue
		}
		ok, err := taggable(svc, it, options)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		key := directoryKey(*it.Identity)
		r := receipts[key]
		r.Candidate = r.Tag == "" && r.PreviousTag == ""
		r.Path = it.Path
		r.Identity = *it.Identity
		receipts[key] = r
		tag := it.ExpiresAt.In(time.Local).Format("01-02") + " burial"
		if options.Language == i18n.Chinese {
			tag = it.ExpiresAt.In(time.Local).Format("01-02") + " 下葬"
		}
		wanted[key] = tag
	}
	return nil
}

// taggable reports whether the scope, Git and Guard rules allow a tag on the item.
func taggable(svc *core.Service, it core.Item, options Options) (bool, error) {
	inScope, err := inFolders(svc.FS, filepath.Dir(it.Path), options.Folders)
	if err != nil {
		return false, err
	}
	if !options.Everywhere && !inScope && it.WatchID == 0 && it.Source != "watch" {
		return false, nil
	}
	if !options.InGit {
		_, found, err := findProject(svc.Store.Context(), filepath.Dir(it.Path))
		if err != nil || found {
			return false, err
		}
	}
	err = svc.Guard.CheckContext(svc.Store.Context(), it.Path, true)
	var refused *core.RefusedError
	if errors.As(err, &refused) {
		return false, nil
	}
	return err == nil, err
}

// relocated follows a receipt's file into the graveyard once it has been buried.
func relocated(state core.State, r directory) string {
	for _, b := range state.Burials {
		if b.Identity != nil && *b.Identity == r.Identity {
			return b.Grave
		}
		if b.IsDir && slices.ContainsFunc(b.Children, func(c core.BuriedChild) bool { return c.Identity != nil && *c.Identity == r.Identity }) {
			rel, err := filepath.Rel(b.Path, r.Path)
			if err == nil && filepath.IsLocal(rel) {
				return filepath.Join(b.Grave, rel)
			}
			return r.Path
		}
	}
	return r.Path
}

func updateTags(path string, r directory, want string, record func(string, string) error) error {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	id, err := winfs.IdentifyHandle(fd, path)
	if err != nil {
		return err
	}
	if id != r.Identity {
		return errors.New("file identity changed; tags left untouched")
	}
	before, err := readTagAttr(fd)
	if err != nil {
		return err
	}
	tags, err := decodeTags(before)
	if err != nil {
		return err
	}
	raw := slices.Clone(tags)
	tags, owned, want, done := planTags(tags, r, want)
	if done {
		return nil
	}
	if !slices.Equal(raw, tags) {
		if err := writeTags(fd, before, tags, func() error { return record(owned, want) }); err != nil {
			return err
		}
	}
	if r.Tag == want && r.PreviousTag == "" && slices.Equal(raw, tags) {
		return nil
	}
	return record("", want)
}

// planTags drops the tag Obituary owns when it changes and adds the wanted one; done means nothing is left to do.
func planTags(tags []string, r directory, want string) (next []string, owned, wanted string, done bool) {
	for _, tag := range []string{r.Tag, r.PreviousTag} {
		if tag != "" && slices.Contains(tags, tag) {
			owned = tag
			break
		}
	}
	if want != "" && slices.ContainsFunc(tags, func(s string) bool { return strings.SplitN(s, "\n", 2)[0] == want }) {
		if owned == want && r.PreviousTag == "" {
			return tags, owned, want, true
		}
		if owned != want {
			want = ""
		}
	}
	if owned != "" && owned != want {
		idx := slices.Index(tags, owned)
		tags = slices.Delete(tags, idx, idx+1)
	}
	if want != "" && !slices.Contains(tags, want) {
		tags = append(tags, want)
	}
	return tags, owned, want, false
}

// writeTags records the intent, then replaces the attribute only if nobody else changed it meanwhile.
func writeTags(fd int, before []byte, tags []string, record func() error) error {
	data, err := encodeTags(tags)
	if err != nil {
		return err
	}
	if err := record(); err != nil {
		return err
	}
	current, err := readTagAttr(fd)
	if err != nil {
		return err
	}
	if !bytes.Equal(before, current) {
		return errors.New("user tags changed; retry")
	}
	if len(tags) == 0 {
		return unix.Fremovexattr(fd, tagAttr)
	}
	return unix.Fsetxattr(fd, tagAttr, data, 0)
}
func readTagAttr(fd int) ([]byte, error) {
	n, err := unix.Fgetxattr(fd, tagAttr, nil)
	if errors.Is(err, noTagAttr) {
		return nil, nil
	}
	if err != nil || n == 0 {
		return nil, err
	}
	buf := make([]byte, n)
	n, err = unix.Fgetxattr(fd, tagAttr, buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}
