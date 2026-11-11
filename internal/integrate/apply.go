package integrate

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
)

// Change is one file Obituary would create, modify or remove.
type Change struct {
	Path      string     `json:"path"`
	Action    string     `json:"action"` // create, update, remove, unchanged
	Detail    string     `json:"detail,omitempty"`
	Commands  [][]string `json:"commands,omitempty"`
	Extension string     `json:"extension,omitempty"` // independent uninstall group
	Implicit  bool       `json:"implicit,omitempty"`  // optional refresh, never blocks hooks
	Manual    bool       `json:"manual,omitempty"`    // exact user steps, not an ownership conflict
	data      []byte
	delete    bool
	// What the file held when the change was planned: Apply refuses to write
	// if it no longer does (the user saved it meanwhile, or another init ran).
	existed      bool
	orig         []byte
	project      string // nonempty for project installs; rechecked before writing
	phase        int    // dependency order: awareness/hooks/references; reversed on uninstall
	claude       *claudeMCPChange
	next         *Change // final receipt after a pending receipt and its dependent file
	location     *integrationLocation
	identity     os.FileInfo
	relocated    *relocatedAssetPlan
	readOnly     bool              // inspection skipped the identity snapshot; never executable
	asset        *managedAssetPlan // owner intent, composed before generating a receipt transaction
	shared       *sharedRegistrationPlan
	preservePath []string         // a conflict confined to this configuration member
	textEdit     *composeTextEdit // exact marked range; avoids shifting across identical block headers
}

// Apply writes planned changes: a .bak copy of every existing file first,
// then an atomic replace. Files Obituary created are removed on uninstall.
//
// Before writing anything it checks that every file still holds what it held
// when the changes were planned (a user may save their settings while the
// prompt is open, or another `obit init` may run): if not, nothing is written.
// Backups never overwrite older backups, so the pre-install original survives
// any number of reinstalls; temporary files have unique names.
// Dependency phases keep references after awareness/hooks on installation,
// and before them on removal. Failures report the changes already applied.
func Apply(changes []Change) error {
	return apply(changes, nil)
}

// afterWrite simulates interruption for this apply only, after a durable write.
//
//nolint:funlen // Preserve the existing write transaction while scoping its interruption hook to one call.
func apply(changes []Change, afterWrite func(string) error) (err error) {
	for _, c := range changes {
		if c.readOnly {
			return fmt.Errorf("%s: read-only installation plan cannot be applied", c.Path)
		}
	}
	changes = slices.Clone(changes)
	slices.SortStableFunc(changes, func(a, b Change) int { return a.phase - b.phase })
	var applied []string
	locks := &sharedApplyLocks{}
	defer locks.close()
	defer func() {
		err = errors.Join(err, PreservedError(changes))
		if err != nil && len(applied) > 0 {
			err = fmt.Errorf("%w; already applied: %s", err, strings.Join(applied, "; "))
		}
	}()
	for i := range changes {
		c := &changes[i]
		if c.Action == "preserve" {
			continue
		}
		p, err := openIntegrationTarget(*c, false)
		if err != nil {
			return err
		}
		if c.location == nil {
			c.location = &p.location
		}
		err = p.checkOriginal(*c)
		if err == nil && c.existed && c.Action != "unchanged" && c.claude == nil && c.relocated == nil {
			err = p.backup(nil, false)
		}
		p.root.Close()
		if err != nil {
			return err
		}
	}
	// Keep receipt pointers so the pending write can pass its new identity to
	// the final receipt; copying it here would retain the obsolete file ID.
	var writes []*Change
	for i := range changes {
		writes = append(writes, &changes[i])
		if changes[i].next != nil {
			writes = append(writes, changes[i].next)
		}
	}
	slices.SortStableFunc(writes, func(a, b *Change) int { return a.phase - b.phase })
	for _, planned := range writes {
		c := *planned
		if c.Action == "unchanged" || c.Action == "preserve" {
			continue
		}
		if err := locks.beforeWrite(c, changes); err != nil {
			return err
		}
		if c.claude != nil {
			// The host owns these writes. Keep its configuration directory pinned
			// until the official CLI has finished, including recovery commands.
			p, err := openIntegrationTarget(c, true)
			if err != nil {
				return err
			}
			if err := p.lockHostDirectory(); err != nil {
				p.root.Close()
				return err
			}
			if beforeIntegrationWrite != nil {
				if err := beforeIntegrationWrite(c.Path); err != nil {
					p.root.Close()
					return err
				}
			}
			steps, err := applyClaudeMCP(c)
			p.root.Close()
			applied = append(applied, steps...)
			if err != nil {
				return err
			}
			applied = append(applied, c.Action+" "+c.Path)
			continue
		}
		p, err := openIntegrationTarget(c, !c.delete)
		if err != nil {
			return err
		}
		err = p.write(c)
		p.root.Close()
		if err != nil {
			return err
		}
		applied = append(applied, c.Action+" "+c.Path)
		if !c.delete && afterWrite != nil {
			if err := afterWrite(c.Path); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkOriginal(c Change) error {
	p, err := openIntegrationTarget(c, false)
	if err != nil {
		return err
	}
	defer p.root.Close()
	return p.checkOriginal(c)
}

func checkOriginalData(c Change, data []byte, err error) error {
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if c.claude != nil {
		entry, err := claudeEntry(data)
		if err != nil {
			return err
		}
		if sameClaudeEntry(entry, c.claude.before) {
			return nil
		}
		return fmt.Errorf("%s: obituary registration changed after planning; run obit init again", c.Path)
	}
	if exists != c.existed || string(data) != string(c.orig) {
		return fmt.Errorf("%s changed after the changes were planned; run obit init again", c.Path)
	}
	return nil
}

func backupPath(path string, n int) string {
	if n == 0 {
		return path + ".bak"
	}
	return fmt.Sprintf("%s.bak.%d", path, n)
}

// Pending reports whether any change would modify a file.
func Pending(changes []Change) bool {
	for _, c := range changes {
		if c.Action != "unchanged" && c.Action != "preserve" {
			return true
		}
	}
	return false
}

// PreservedError reports extensions deliberately left untouched on uninstall.
func PreservedError(changes []Change) error {
	var errs []error
	for _, c := range changes {
		if c.Action == "preserve" && !c.Implicit {
			errs = append(errs, fmt.Errorf("%s preserved: %s", c.Path, c.Detail))
		}
	}
	return errors.Join(errs...)
}
