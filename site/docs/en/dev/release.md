# Releases and mirrors

Check three sources at the version tag: `CHANGELOG.md` for behavior changes, `release/notes-VERSION.md` for user-facing scope, and `.goreleaser.yaml` for actual archive contents. A GUI present in one archive is not evidence that another platform ships it.

`release.yml` responds to version tags starting with `v`. GoReleaser creates six archives across Windows, Linux and macOS on amd64 and arm64, plus SHA-256 checksums. Only the amd64 Windows archive includes the GUI. Separate desktop workflows build the Linux `.deb` and the universal macOS DMG. Building an artifact, attaching it to a release and listing it in a package manager are distinct: check the version and download checksums for Scoop, WinGet and the Homebrew cask separately.

After the Scoop bucket and Homebrew tap both point to the published release, run **Scoop and Homebrew CLI acceptance** (`channel-acceptance.yml`) manually in Actions. Enter a `version` such as `1.6.2`, without the `v`. Both jobs must pass: install from the channel, verify the index and executable versions, register, bury and restore a fixture, then uninstall with the package manager and check that the executables are gone while the isolated ledger and non-empty graveyard are unchanged. Record both jobs' log artifacts in the release evidence. The Homebrew job covers the CLI cask, not the macOS desktop DMG.

The website deploys independently of binaries. `pages.yml` builds `dist/` and `dist-cn/` from one source tree; the mainland installation examples use mirror scripts. `mirror-latest.yml` verifies the release archives and checksums before publishing the mirror index. Transport checksums are not independent publisher signatures. Verify the served index and downloads, not just the homepage.
