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
	"github.com/woodfishhhh/obituary/internal/regtxn"
	"github.com/woodfishhhh/obituary/internal/textutil"
	"github.com/woodfishhhh/obituary/internal/userdirs"
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

func (p *plan) preflightData(out io.Writer) error {
	if len(p.pending) != 0 {
		return errors.New("data retained: pending graveyard recovery requires attention before --purge-data")
	}
	if err := p.cleanMetadata(false); err != nil {
		return err
	}
	return p.purgeGraves(out, true)
}

func (p *plan) purgeGraves(out io.Writer, preview bool) error {
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
		results, err := p.svc.PurgeRefs(refs, preview)
		for _, r := range results {
			if preview && r.Error == "" {
				continue
			}
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
	return nil
}

func (p *plan) purgeData(out io.Writer) error {
	// Stopping services and removing integrations can change the data directory.
	if len(p.state.Burials) > 0 {
		if err := p.cleanMetadata(false); err != nil {
			return err
		}
	}
	if err := p.purgeGraves(out, false); err != nil {
		return err
	}
	if err := p.requireEmptyGraveyard(); err != nil {
		return err
	}
	if err := p.cleanMetadata(true); err != nil {
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
	"explorer.json", "explorer.json.tmp", "explorer.lock", "explorer.log", "explorer.log.1", "explorer.stamp",
	"update.json", "update.json.tmp", "update.lock",
	"proposal-log.json", "proposal-log.json.tmp", "proposal-log.lock",
	"obitd.log", "obitd.stop", "obitd.paused", "obitd.process.json", "hook.log", "hook.log.1", "hook.log.lock",
	"hook-audit.log", "hook-audit.log.1", "hook-audit.log.lock", "guard-rm.json", "guard-rm.lock",
	"observations.jsonl", "observations.lock", "trusted_rules.json", "trusted_rules.json.lock",
	"manifest-recovery.json", "manifest-recovery.lock", "proposals.lock", "delete-guard.jsonl",
}

// os.Root supplies handle-relative operations on Windows (Go 1.26). Unknown
// files, links, and nonempty subdirectories keep the ledger intact for review.
func (p *plan) cleanMetadata(remove bool) error {
	home := p.svc.Store.Home
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
	entries, keepConfig, err := p.metadataEntries(root, opened, remove)
	if err != nil || !remove {
		return err
	}
	if err := removeMetadataEntries(root, entries); err != nil {
		return err
	}
	current, err := parent.Lstat(filepath.Base(home))
	if err != nil {
		return err
	}
	if !os.SameFile(info, current) {
		return errors.New("data directory changed during cleanup")
	}
	if keepConfig {
		return nil
	}
	return parent.Remove(filepath.Base(home)) // empty directory only
}

func (p *plan) metadataEntries(root *os.Root, home os.FileInfo, remove bool) ([]os.DirEntry, bool, error) {
	dir, err := root.Open(".")
	if err != nil {
		return nil, false, err
	}
	entries, err := dir.ReadDir(-1)
	dir.Close()
	if err != nil {
		return nil, false, err
	}
	var keepConfig bool
	if cfg, err := os.Stat(p.config); err == nil && os.SameFile(home, cfg) {
		entries = slices.DeleteFunc(entries, func(e os.DirEntry) bool {
			if e.Name() == "config.toml" && e.Type().IsRegular() {
				keepConfig = true
				return true
			}
			return false
		})
	}
	if len(entries) == 0 && keepConfig {
		return nil, true, nil // A repeated uninstall has nothing left to remove.
	}
	if err := core.VerifyDataOwner(root); err != nil {
		return nil, false, err
	}
	var burials []core.Burial
	if !remove {
		burials = p.state.Burials
	}
	for _, entry := range entries {
		if err := checkMetadata(root, entry.Name(), burials); err != nil {
			return nil, false, err
		}
	}
	return entries, keepConfig, nil
}

func removeMetadataEntries(root *os.Root, entries []os.DirEntry) error {
	for _, entry := range entries {
		if entry.Name() == "sessions" {
			if err := removeSessions(root); err != nil {
				return err
			}
		}
		if entry.Name() == registryLocks {
			if err := removeRegistryLocks(root); err != nil {
				return err
			}
		}
		if err := root.Remove(entry.Name()); err != nil {
			return err
		}
	}
	return nil
}

func checkMetadata(root *os.Root, name string, burials []core.Burial) error {
	info, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if info.IsDir() && name == "sessions" {
		_, err := sessionFiles(root)
		return err
	}
	if info.IsDir() && name == registryLocks {
		_, err := registryLockFiles(root)
		return err
	}
	if info.IsDir() && name == "graveyard" {
		return checkLocalGraves(root, burials)
	}
	if info.Mode().IsRegular() && (slices.Contains(metadataNames, name) || core.OwnedLedgerFile(root, name)) {
		return nil
	}
	return fmt.Errorf("data retained: unknown file, link or nonempty directory %s", filepath.Join(root.Name(), name))
}

// Core verifies each listed grave and its manifest. Anything else in this
// directory would prevent its final empty-directory removal after the purge.
func checkLocalGraves(root *os.Root, burials []core.Burial) error {
	dir, err := root.Open("graveyard")
	if err != nil {
		return err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return err
	}
	known := make(map[string]bool, len(burials))
	for _, b := range burials {
		if b.Kind != core.KindContainer {
			grave := filepath.Dir(b.Grave)
			known[grave] = true
			known[grave+".json"] = true
		}
	}
	for _, entry := range entries {
		path := filepath.Join(root.Name(), "graveyard", entry.Name())
		if !known[path] {
			return fmt.Errorf("data retained: unknown graveyard entry %s", path)
		}
	}
	return nil
}

const registryLocks = "registry-locks"

// Installers keep per-user locks in the default data directory
// (userdirs.RegistryLocks). It is owned only there, and only while it holds
// nothing but their empty lock files.
func registryLockFiles(root *os.Root) ([]string, error) {
	retained := fmt.Errorf("data retained: unrecognized registry lock directory %s", filepath.Join(root.Name(), registryLocks))
	path, err := userdirs.RegistryLocks()
	if err != nil {
		return nil, errors.Join(retained, err)
	}
	want, err := os.Lstat(path)
	if err != nil {
		return nil, errors.Join(retained, err)
	}
	if got, err := root.Lstat(registryLocks); err != nil || !os.SameFile(got, want) {
		return nil, errors.Join(retained, err)
	}
	dir, err := root.Open(registryLocks)
	if err != nil {
		return nil, err
	}
	entries, err := dir.ReadDir(-1)
	dir.Close()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		name := registryLocks + "/" + entry.Name()
		info, err := root.Lstat(name)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Size() != 0 || !regtxn.IsLockName(entry.Name()) {
			return nil, fmt.Errorf("data retained: unrecognized registry lock %s", filepath.Join(root.Name(), name))
		}
		names = append(names, name)
	}
	return names, nil
}

func removeRegistryLocks(root *os.Root) error {
	names, err := registryLockFiles(root)
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := root.Remove(name); err != nil {
			return err
		}
	}
	return nil
}
