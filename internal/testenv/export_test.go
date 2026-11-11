//go:build windows

package testenv

import "io"

var ReapScratch = reapScratch
var WriteScratchOwner = writeScratchOwner
var ScratchOwnerAlive = scratchOwnerAlive

type ScratchOwner = scratchOwner

const ScratchOwnerFile = scratchOwnerFile

func DesktopChanges(classes, run string, shortcuts []string, change func(), out io.Writer) (bool, error) {
	locations := desktopLocations{classes: classes, run: run, shortcuts: shortcuts}
	before, err := snapshotDesktop(locations)
	if err != nil {
		return false, err
	}
	change()
	return checkDesktop(before, func() (desktopFootprint, error) { return snapshotDesktop(locations) }, out), nil
}

func GraveyardChanges(root string, run func()) []string {
	before := graveyardEntries(root)
	run()
	return newGraveyardEntries(before, graveyardEntries(root))
}
