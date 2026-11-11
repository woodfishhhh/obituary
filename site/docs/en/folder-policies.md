# Folder policies

A folder policy gives everything that lands in a folder its own lifespan. Downloads is the usual one:

```powershell
obit watch C:\Users\you\Downloads 2w        # files that land in Downloads are buried two weeks later
obit watch --list                           # folder policies, their exemptions and quiet periods
obit watch --remove C:\Users\you\Downloads  # remove a policy and the lifespans it registered
```

## How it takes effect

- **The policy itself needs your approval.** `obit watch` creates a proposal; it takes effect after `obit approve` or approval in the small window's "For you" page.
- **Ten quiet minutes first.** A file is buried once it has expired and has been left untouched for 10 minutes.
- **Subfolders get their own proposal.** An expired subfolder only produces a proposal and is buried after you approve it.
- Removing a policy cancels the lifespans it registered.

## Dates in Explorer

Tracked items in Downloads, on the Desktop and in folders under a policy show in Explorer as "name (11-13 burial)" while the real file name stays the same.

- The dates come from a hidden `desktop.ini` in the folder.
- `[explorer]` in `config.toml` turns them off (`enabled = false`) or on everywhere (`everywhere = true`).
- Explorer caches folder views, so after extending a lifespan the same window may show the old date for up to a minute.
