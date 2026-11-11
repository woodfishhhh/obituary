---
name: obituary
description: Give files a lifespan and delete them restorably with Obituary (obit). Use when creating scratch files, debug scripts, logs, screenshots or other artefacts that should not outlive the task, when deleting files or directories, when a command was denied by Obituary's delete guard, or when something deleted earlier needs to come back.
---

# Obituary (讣告)

Obituary gives files and directories a lifespan. When the lifespan runs out they are buried in a graveyard, where they stay restorable until they are purged (after 7 days by default). The CLI is `obit`. Commands that report state or results (`list`, `graveyard`, `proposals`, `stats`, `mark`, `bury`, `resurrect` and similar) accept `--json`; the deletion wrappers below do not.

## Artefacts that should clean themselves up

- Scratch space: `dir=$(obit mktemp 1d)` returns a new directory that is buried after a day. Inside a Git project it lives in `.obit/tmp/`, which Git ignores.
- A file or directory you created for the task (debug script, log, screenshot, test output): `obit mark <path> <lifespan>`. Lifespans look like `30m`, `2h`, `3d`, `1w`, and combine (`1d12h`).
- `obit list` shows what has a lifespan and how long is left; `obit extend <id> <lifespan>` gives more time; `obit spare <id>` stops tracking and keeps the file.
- If a command reports `commit unconfirmed`, its change may already have taken effect: check `obit list` (or `obit graveyard`) before running it again, since each `obit extend` adds more time.

## Deleting

- `obit bury <path>...` moves things into the graveyard right away instead of deleting them.
- A Docker container you start for a task can get a lifespan too: `obit docker add <name-or-id> <lifespan>`. At expiry it is stopped (its writable layer stays), `obit resurrect <id>` starts it again, and it is removed only after the retention period; volumes and images are never touched. Containers started with `--rm` are refused.
- `obit rm`, `obit rmdir` and `obit git-clean` accept the common non-interactive arguments of `rm`, `rmdir` and `git clean` (`-r`, `-f`, `-d`, `-x` and the like); interactive or unusual options such as `rm -i` or `rmdir -p` are refused with an error rather than guessed at. `obit remove-item` takes a single literal path like `Remove-Item -LiteralPath`. Everything they remove goes to the graveyard.
- When Obituary's delete guard is enabled, a deletion command is denied and the reason names the equivalent `obit` command. Running that command does the same deletion, restorably. The guard only stops commands it fully understands; anything else runs unchanged.

## Bringing things back

- `obit graveyard` lists buried items with their numbers and how long they remain restorable; `obit graveyard --here` lists only those whose original path is in the current directory.
- If a file or directory you need is missing, it may have been buried, by you, the user or its lifespan. Obituary adds a grave notice when a tool call touches a buried path; otherwise run `obit graveyard --here` before recreating it. If it is there, `obit resurrect <id>` puts it back (a notice for a path inside a buried directory restores the whole directory).
- `obit resurrect <id>` puts an item back where it was. It never overwrites: if the original path is taken, the item comes back under a suffixed name, and the output says where.
- `obit resurrect --last` undoes the latest deletion: everything one `obit bury` or `obit rm` command buried comes back together. Inside an agent session it undoes only that session's deletions (the session is recognized automatically in Claude Code; `OBIT_SESSION_ID` overrides it), falling back to deletions in the current Git project that recorded no session. Run from a plain terminal, it undoes the latest deletion in the current Git project, whichever session made it. Other projects and expiry are never picked, and it never falls back to an older batch. When nothing matches it reports an error, and `obit graveyard` shows the numbers.

## Decisions that belong to the user

Some actions are the user's to take, and Obituary is built around that:

- Proposals: when Obituary's hooks would register artefacts automatically for the first time in a project, or a batch is large, they create a proposal instead of acting. An explicit `obit mark` or `obit mktemp` is not a proposal: it takes effect at once. `obit proposals` shows them. Approving (`obit approve <id>`) or rejecting (`obit reject <id>`) is the user's decision; the right move is to describe the proposal to the user and let them answer.
- Project rules: a repository's `.obitrules` takes effect only after the user reviews it and runs `obit trust`.
- Purging: `obit purge` deletes graveyard items permanently. It is the user's call. `obit purge <id>` purges that item at once, before its retention ends; it is also how the user abandons an interrupted resurrection that can no longer be verified.
- In `obit resurrect` and `obit purge`, a bare number (`5` or `#5`) always means an ID; a file whose name is a number is written as `./5` or as an absolute path.

## Facts worth knowing

- Files tracked by Git are never given a lifespan automatically, and an automatic lifespan is re-checked against Git before burial.
- Burying a directory takes everything inside it; a directory that contains a `.git` repository needs `--force` with `obit bury`, while `obit rm -rf` buries it as `rm -rf` would delete it.
- If the user has set `mode = "recycle"` in Obituary's configuration, buried and expired items go to the system Recycle Bin instead of the graveyard; `obit resurrect` cannot bring those back, and they are restored from the Recycle Bin. `obit config` shows the mode in effect.
- `obit stats` shows how much has been buried, purged and resurrected; `obit doctor` checks the installation.
