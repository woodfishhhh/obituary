# Automatic lifespans

## The agent's own output goes in, your files wait for you

By default Obituary sorts automatic registration into tiers (`[proposals] mode = "tiered"`). Three kinds go into Obituary directly, without waiting for you; the agent is told afterwards, and they are listed in `obit proposals` and under Handled automatically on the window's Decisions page:

1. **Files the agent created in its session**: the hook watched it write them, they were not there when the session started, and the file system created them after it started. Rule matches get the rule's lifespan.
2. **Regenerable output**: Git-ignored dependencies, build output and caches the session left (`node_modules`, `dist`, `.turbo` and the like), buried after 3 days.
3. **Docker containers the session started for the project**; see [Docker containers](./docker).

Only what the agent did not create becomes a proposal: files that were there before the session, or files the hook did not see the agent write. The agent passes the proposal on to you; `obit approve <id>` registers it and lets rule matches in that project register directly from then on; `obit reject <id>` means never register automatically in this project.

```powershell
obit proposals           # proposals waiting for you, and what was registered without asking
obit approve 3           # approve: register what it lists
obit reject 3            # reject
obit spare 12            # undo an automatic registration: it stays where it is, without a lifespan
obit extend 12 1w        # keep it longer
```

To hand everything to the agent, switch to Fully automatic on the window's Decisions page, or set `[proposals] mode = "auto"` in `config.toml`: whatever automatic registration finds goes in directly and you are told afterwards. To be asked about everything first, set `"ask"`. After a switch the background runner sorts the proposals still waiting under the new mode.

- **Large things** (over 1 GB or 200 files) the agent did not create are proposed as well.
- **Session diff.** A snapshot is taken when a session starts (kept in the data directory, nothing is added to your repository) and compared when it ends. Items covered by rules are handled as above. Of the rest, only what Git ignores and tools regenerate is registered. New files Git does not ignore are code you have not committed yet, and ignored local configuration such as `.env` cannot be regenerated, so neither is ever registered automatically or proposed.
- **End-of-session tidy-up.** When a Claude Code or Codex agent is about to stop, Obituary lists the files it created this session that nothing has sorted and asks it to give debug scripts, logs and screenshots a lifespan or bury them, keeping the deliverables. At most once per session, and silent when there is nothing; other agents hear it at the next session start. `[hooks] session_cleanup = false` turns it off.
- **Git is asked again before burial.** Anything registered by rule or proposal is not buried if it has been committed by the time it expires.
- **Proposals expire on their own.** Entries that were committed or deleted are dropped, and proposals with nothing left are withdrawn (recorded in the audit log, not counted as a rejection).

## Rules

```powershell
obit rules                       # the rules in effect here (built-in plus a trusted .obitrules)
obit rules check debug.log out/  # preview what would be registered and by which rule (changes nothing)
obit rules apply debug.log       # register by rule
```

Built-in rules cover common agent clutter: `.playwright-mcp/`, `test-results/`, `__pycache__/`, `*.log` and more. A `.obitrules` file at the project root uses `.gitignore` syntax with a lifespan after each pattern:

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
