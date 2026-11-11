package integrate

// ownedMarker marks files obit init owns outright; they are only ever
// replaced or deleted while it is still there.
const ownedMarker = "<!-- obit:owned — written by obit init; obit init --uninstall removes this file -->"

// Awareness is OBIT.md, the instructions `obit init` gives the agent.
// It is factual and short; hooks add live details.
const Awareness = ownedMarker + `

# Obituary (讣告)

Obituary gives files a lifespan. When it runs out they are buried in a
graveyard, restorable with ` + "`obit resurrect <id>`" + ` until they are purged
(after 7 days by default; the user's config may change this). When the user has
configured recycle mode, buried items go to the system Recycle Bin instead and
` + "`obit resurrect`" + ` cannot restore them.

When the delete guard is enabled, deletion commands are denied with an equivalent obit command as a suggestion.

- Scratch space that should clean itself up: ` + "`dir=$(obit mktemp 1d)`" + `.
- A debug script, log, screenshot or other artefact of yours:
  ` + "`obit mark <path> <lifespan>`" + ` (lifespans like 2h, 3d, 1w).
- Deleting something: ` + "`obit bury <path>`" + ` instead of ` + "`rm -rf`" + `; it stays restorable.
- A file or folder you need is missing: it may be in the graveyard. Obituary
  tells you when a tool call touches a buried path; otherwise check
  ` + "`obit graveyard --here`" + `, and ` + "`obit resurrect <id>`" + ` puts it back.
- ` + "`obit list`" + `, ` + "`obit graveyard`" + `, ` + "`obit extend <id> <lifespan>`" + `, ` + "`obit spare <id>`" + `.

Files tracked by Git are never registered automatically. Before automatic
registration starts in a project for the first time, Obituary makes a proposal
instead of acting (explicit ` + "`obit mark`" + `, ` + "`obit mktemp`" + ` and ` + "`obit bury`" + ` act directly): a
pending proposal is described at the start of a session or after a write, and
only the user decides on it
(` + "`obit approve <id>`" + ` / ` + "`obit reject <id>`" + `). The same applies to ` + "`obit trust`" + `,
which enables a project's .obitrules after the user has reviewed it.
`
