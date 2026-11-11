//go:build windows

package testenv

var ReapScratch = reapScratch
var WriteScratchOwner = writeScratchOwner
var ScratchOwnerAlive = scratchOwnerAlive

type ScratchOwner = scratchOwner

const ScratchOwnerFile = scratchOwnerFile

func GraveyardChanges(root string, run func()) []string {
	before := graveyardEntries(root)
	run()
	return newGraveyardEntries(before, graveyardEntries(root))
}
