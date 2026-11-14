# What goes into Obituary

## The agent and you decide; Obituary buries on time

The agent and you decide what goes into Obituary (the default, `[proposals] mode = "off"`):

- **The agent registers its own output.** It gives the debug scripts, logs and screenshots it writes a lifespan with `obit mark <path> 2d`, takes scratch space from `obit mktemp 1d`, and buries what it is unsure about with `obit bury`. The instructions `obit init` writes for the agent say so.
- **You add from the context menu.** On Windows, right-click files or folders in Explorer (several at once is fine) → "Add to Obituary" → 1, 7 or 30 days, or "Bury now (recoverable)". See [Window, tray and context menu](./desktop).
- **Folder policies** give a folder such as Downloads a lifespan; see [Folder policies](./folder-policies).

Whatever is in is buried when due and can be resurrected as it was from the graveyard for 7 days. Nothing else goes in by itself.

- **End-of-session tidy-up.** When a Claude Code or Codex agent is about to stop, Obituary lists the files it created this session that nothing has sorted and asks it to give debug scripts, logs and screenshots a lifespan or bury them, keeping the deliverables. At most once per session, and silent when there is nothing; other agents hear it at the next session start. `[hooks] session_cleanup = false` turns it off.
- **Upgrading from an older version** switches this setting off once (whatever you set afterwards stands), and the dependencies, build output and caches registered automatically before are spared: they stay where they are, without a lifespan.

## Turn automatic registration on when you want it

Switch it in the window's Settings or Decisions page, or set `[proposals] mode` in `config.toml`:

- `"tiered"`: files the agent created in its session (the hook watched it write them, they were not there when the session started, and the file system created them after it started) and Docker containers the session started for the project (see [Docker containers](./docker)) go in directly; the agent is told afterwards, and they are listed in `obit proposals` and under Handled automatically on the Decisions page. The rest becomes a proposal: files that were there before the session, or files the hook did not see the agent write. `obit approve <id>` registers it and lets rule matches in that project register directly from then on; `obit reject <id>` means never register automatically in this project.
- `"auto"`: whatever automatic registration finds goes in directly and you are told afterwards.
- `"ask"`: everything waits as a proposal first.

```powershell
obit proposals           # proposals waiting for you, and what was registered without asking
obit approve 3           # approve: register what it lists
obit reject 3            # reject
obit spare 12            # undo an automatic registration: it stays where it is, without a lifespan
obit extend 12 1w        # keep it longer
```

- **Dependencies, build output and caches** (`node_modules`, `dist`, `.turbo`, `__pycache__` and the like) are never registered automatically or proposed in any mode; `obit mark` them yourself to clean them up.
- **Large things** (over 1 GB or 200 files) the agent did not create are proposed as well.
- **Session diff.** A snapshot is taken when a session starts (kept in the data directory, nothing is added to your repository) and compared when it ends. Rule matches are handled by the mode; new files no rule covers are neither registered nor proposed.
- **After a switch** the background runner sorts the proposals still waiting under the new mode.
- **Git is asked again before burial.** Anything registered by rule or proposal is not buried if it has been committed by the time it expires.
- **Proposals expire on their own.** Entries that were committed or deleted are dropped, and proposals with nothing left are withdrawn (recorded in the audit log, not counted as a rejection).

## Rules

```powershell
obit rules                       # the rules in effect here (built-in plus a trusted .obitrules)
obit rules check debug.log out/  # preview what would be registered and by which rule (changes nothing)
obit rules apply debug.log       # register by rule
```

`obit rules apply` registers by rule at any time; with automatic registration on, the hooks register by these rules too. Built-in rules cover common agent clutter: `.playwright-mcp/`, `test-results/`, `*.log` and more; backups left when editing (`*.bak`, `*.bak.*`, `*.orig`, `*.rej`, `*.old`, `*.backup`, `*~`) are kept 7 days, while LevelDB's `LOG.old` is left alone. A `.obitrules` file at the project root uses `.gitignore` syntax with a lifespan after each pattern:

```
output/**        7d      # every item under output gets its own lifespan
!output/keep/**          # never touch (negation wins, even over registering a parent folder)
test-results/    off     # turn off a built-in rule
```

Unlike `.gitignore`: negation wins rather than the last match; on Windows matching ignores case; an invalid pattern (such as an unclosed `[`) is an error naming the file and line, and automatic registration in that project pauses.

Durations: `s` `m` `h` `d` `w`, combinable (`1d12h`).

## Hard rules for automatic registration

- Only untracked items inside a Git work tree.
- A folder with even one tracked file is never touched.
- If Git cannot answer, skip.
- An existing lifespan is never shortened.
- A project you declined is never registered directly, in any mode; folder policies and trusting `.obitrules` always wait for you.

## Project rules must be trusted first

A repository's own `.obitrules` has no effect until you have reviewed it and run `obit trust`, so someone else's repository cannot use rules to take your private files. Trust records the file's SHA-256; any change needs a new review. Non-interactive sessions need `--yes`; in CI you can set `OBIT_TRUST_PROJECT_RULES=1`.

## A temporary folder that buries itself

```powershell
$dir = obit mktemp 1d            # .obit/tmp/ in a project (not in Git), otherwise the system temp folder
```

In bash: `dir=$(obit mktemp 1d)`.
