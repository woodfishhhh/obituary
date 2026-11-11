package uninstall

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/textutil"
)

func (p *plan) printData(out io.Writer, purge bool) {
	action := "KEEP"
	if purge {
		action = "PURGE (permanent; requires confirmation)"
	}
	fmt.Fprintf(out, "  %s data %s\n  keep user configuration %s\n", action, p.svc.Store.Home, p.config)
	fmt.Fprintf(out, "  graveyard: %s; %s; local root %s\n",
		textutil.Count(len(p.state.Burials), "recorded entry", "recorded entries"), textutil.Count(len(p.pending), "pending record"), filepath.Join(p.svc.Store.Home, "graveyard"))
	for _, b := range p.state.Burials {
		fmt.Fprintf(out, "    #%d %s -> %s\n", b.ID, b.Path, b.Grave)
	}
	for _, pending := range p.pending {
		fmt.Fprintf(out, "    pending #%d %s (%s)\n", pending.Number(), pending.Manifest, pending.Status)
	}
}

func (p *plan) purgeData(out io.Writer) error {
	if len(p.pending) != 0 {
		return errors.New("data retained: pending graveyard recovery requires attention before --purge-data")
	}
	st, err := p.svc.Store.Inspect()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(st.Burials, p.state.Burials) {
		return errors.New("graveyard changed since the plan; review --dry-run again")
	}
	refs := make([]string, 0, len(st.Burials))
	for _, b := range st.Burials {
		refs = append(refs, fmt.Sprint(b.ID))
	}
	if len(refs) > 0 {
		results, err := p.svc.PurgeRefs(refs, false)
		for _, r := range results {
			fmt.Fprintf(out, "  %s %s %s\n", r.Ref, r.Action, r.Detail)
			if r.Error != "" {
				fmt.Fprintln(out, "    error:", r.Error)
			}
			if r.Warning != "" {
				fmt.Fprintln(out, "    warning:", r.Warning)
			}
		}
		if err != nil || core.GraveFailed(results) {
			return errors.Join(err, cmdutil.ErrReported)
		}
	}
	if err := p.requireEmptyGraveyard(); err != nil {
		return err
	}
	if err := removeMetadata(p.svc.Store.Home); err != nil {
		return err
	}
	fmt.Fprintln(out, "Removed owned data:", p.svc.Store.Home)
	return nil
}

func (p *plan) requireEmptyGraveyard() error {
	st, err := p.svc.Store.Inspect()
	if err != nil {
		return err
	}
	pending, err := p.svc.PendingGraves()
	if err != nil {
		return err
	}
	if len(st.Burials) != 0 || len(pending) != 0 {
		return errors.New("graveyard still contains entries; retaining data for recovery")
	}
	return nil
}

// Metadata names come from the existing store/log writers. Do not recursively
// delete Home: OBIT_HOME may be a shared directory containing user files.
var metadataNames = []string{
	core.DataOwnerFile,
	"store.json", "store.lock", "store.json.tmp", "sweep.lock", "graveyard.lock",
	"grave-notice-paths.json", "grave-notice-paths.json.tmp", "grave-notice-paths.lock",
	"audit.jsonl", "audit.jsonl.1", "audit.lock", "audit-outbox.lock",
	"explorer.json", "explorer.json.tmp", "explorer.lock", "explorer.log", "explorer.stamp",
	"update.json", "update.json.tmp", "update.lock",
	"obitd.log", "obitd.stop", "obitd.paused", "obitd.process.json", "hook.log", "hook.log.1",
	"hook-audit.log", "hook-audit.log.1", "guard-rm.json", "guard-rm.lock",
	"observations.jsonl", "observations.lock", "trusted_rules.json", "trusted_rules.json.lock",
	"manifest-recovery.json", "manifest-recovery.lock", "proposals.lock", "delete-guard.jsonl",
}

// os.Root supplies handle-relative operations on Windows (Go 1.26). Unknown
// files, links, and nonempty subdirectories keep the ledger intact for review.
func removeMetadata(home string) error {
	info, err := os.Lstat(home)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("data path is not an ordinary directory: %s", home)
	}
	parent, err := os.OpenRoot(filepath.Dir(home))
	if err != nil {
		return err
	}
	defer parent.Close()
	root, err := parent.OpenRoot(filepath.Base(home))
	if err != nil {
		return err
	}
	defer root.Close()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		return errors.Join(err, errors.New("data directory changed before cleanup"))
	}
	if err := removeMetadataEntries(root); err != nil {
		return err
	}
	current, err := parent.Lstat(filepath.Base(home))
	if err != nil {
		return err
	}
	if !os.SameFile(info, current) {
		return errors.New("data directory changed during cleanup")
	}
	return parent.Remove(filepath.Base(home)) // empty directory only
}

func removeMetadataEntries(root *os.Root) error {
	if err := core.VerifyDataOwner(root); err != nil {
		return err
	}
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	entries, err := dir.ReadDir(-1)
	dir.Close()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := checkMetadata(root, entry.Name()); err != nil {
			return err
		}
	}
	for _, entry := range entries {
		if entry.Name() == "sessions" {
			if err := removeSessions(root); err != nil {
				return err
			}
		}
		if err := root.Remove(entry.Name()); err != nil {
			return err
		}
	}
	return nil
}

func checkMetadata(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if info.IsDir() && name == "sessions" {
		_, err := sessionFiles(root)
		return err
	}
	if info.IsDir() && name == "graveyard" {
		dir, err := root.Open(name)
		if err != nil {
			return err
		}
		defer dir.Close()
		names, err := dir.Readdirnames(-1)
		if err == nil && len(names) == 0 {
			return nil
		}
	}
	if info.Mode().IsRegular() && slices.Contains(metadataNames, name) {
		return nil
	}
	return fmt.Errorf("data retained: unknown file, link or nonempty directory %s", filepath.Join(root.Name(), name))
}
