package integrate

// shippedAwareness holds the SHA-256 of every earlier Awareness that a
// release or main wrote, with "\r\n" folded to "\n". A file equal to one of
// them is still Obituary's own complete document, so an upgrade refreshes it
// instead of reporting it as someone else's. When Awareness changes, add the
// previous hash here (TestAwarenessChangeKeepsOwnership names it).
var shippedAwareness = map[string]bool{
	"ac1a01aeba00e65e73b5b90058b5aa7c5e8c562b62bd74026a33e5ce62828cf9": true, // 1.0.0 to 1.1.1
	"a8f4fee772231351412bde9bf1fdb18ae5368cb12675f66f255a95c38f6f192b": true, // main between 1.1.1 and 1.2.0
	"4f65738ff0b18132e0e7a19e933d51c32d8d36cdd0613bace9d8bf5d2600de2d": true, // 1.2.0 and 1.2.1
	"b4f7e44a745c97f39d22cc9c5ecf6d0f3faae34bc25f3641d3333ef4530112d8": true, // 1.3.0
}

// ownedMarker marks files obit init owns outright; they are only ever
// replaced or deleted while it is still there.
const ownedMarker = "<!-- obit:owned — written by obit init; obit init --uninstall removes this file -->"

// Awareness is OBIT.md, the instructions `obit init` gives the agent.
// It is factual and short; hooks add live details.
const Awareness = ownedMarker + `

# Obituary (讣告) — the trash bin for AI agents

Obituary gives files a lifespan. When it runs out they are buried in a
graveyard, restorable with ` + "`obit resurrect <id>`" + ` until they are purged
(after 7 days by default; the user's config may change this). When the user has
configured recycle mode, buried items go to the system Recycle Bin instead and
` + "`obit resurrect`" + ` cannot restore them.

When the delete guard is enabled, deletion commands are denied with an equivalent obit command as a suggestion.

Nothing goes into Obituary by itself; what does is your decision:

- Scratch space that should clean itself up: ` + "`dir=$(obit mktemp 1d)`" + `.
- A debug script, log, screenshot or other artefact of yours, and a backup you
  make before editing a file (*.bak and the like):
  ` + "`obit mark <path> <lifespan>`" + ` (lifespans like 2h, 3d, 1w).
- Deleting something: ` + "`obit bury <path>`" + ` instead of ` + "`rm -rf`" + `; it stays restorable.
- A file or folder you need is missing: it may be in the graveyard. Obituary
  tells you when a tool call touches a buried path; otherwise check
  ` + "`obit graveyard --here`" + `, and ` + "`obit resurrect <id>`" + ` puts it back.
- ` + "`obit list`" + `, ` + "`obit graveyard`" + `, ` + "`obit extend <id> <lifespan>`" + `, ` + "`obit spare <id>`" + `.

On Windows the user can also give files a lifespan or bury them from the
Explorer context menu. Files tracked by Git, and dependencies, build output and
caches (node_modules, dist, __pycache__ and the like), are never registered
automatically. If the user has turned automatic registration on, files you
create in a session (the hook sees the write) and containers the session starts
for the project may get a lifespan by themselves; Obituary says so afterwards,
and ` + "`obit spare <id>`" + ` undoes one. Anything else it finds becomes a proposal
that only the user decides on (` + "`obit approve <id>`" + ` / ` + "`obit reject <id>`" + `). The
same applies to ` + "`obit trust`" + `, which enables a project's .obitrules after the
user has reviewed it.

Once per session, when you stop, Obituary may list the files you created that
still have no lifespan and that Git does not track: scratch among them gets
` + "`obit mark <path> <lifespan>`" + ` or ` + "`obit bury <path>`" + `; deliverables stay.
`
