# Automatic lifespans

## Consent first

The first time Obituary wants to register something by rule in a project, it creates a proposal instead of acting. The agent passes it on to you. `obit approve <id>` lets that project register automatically from then on; `obit reject <id>` means never ask about this project again.

```powershell
obit proposals           # proposals waiting for you
obit approve 3           # approve: register what it lists
obit reject 3            # reject
```

- **Large things** (over 1 GB or 200 files) are proposed as well.
- **Session diff.** A snapshot is taken when a session starts (kept in the data directory, nothing is added to your repository) and compared when it ends. Items covered by rules are registered. Of the rest, only what Git ignores and tools regenerate (dependencies, build output, caches such as `node_modules`, `dist`, `.turbo`) is proposed; once approved it is buried after 3 days and stays restorable. New files Git does not ignore are code you have not committed yet, and ignored local configuration such as `.env` cannot be regenerated, so neither ever appears in a proposal, and the agent can safely recommend approving.
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

## Project rules must be trusted first

A repository's own `.obitrules` has no effect until you have reviewed it and run `obit trust`, so someone else's repository cannot use rules to take your private files. Trust records the file's SHA-256; any change needs a new review. Non-interactive sessions need `--yes`; in CI you can set `OBIT_TRUST_PROJECT_RULES=1`.

## A temporary folder that buries itself

```powershell
$dir = obit mktemp 1d            # .obit/tmp/ in a project (not in Git), otherwise the system temp folder
```

In bash: `dir=$(obit mktemp 1d)`.
