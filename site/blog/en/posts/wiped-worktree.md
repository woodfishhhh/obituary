---
title: 'How backticks in release notes wiped our main checkout'
description: 'On the evening of 19 November 2026 a Codex worker put release notes containing backticks into a shell command and emptied the project''s main checkout. The delete guard saw only text; the failure happened when Codex on Windows handed the command to another shell. How it happened, our first wrong diagnosis, the 1.4.3 rule, and the sandbox we tested and set aside.'
developmentDate: '2026-11-19'
translationKey: wiped-worktree
sequence: 17
draft: false
---

At 22:57 on 19 November 2026, we were releasing Obituary 1.4.2. The Codex worker updating our documentation submitted a patch through a shell command containing a quoted heredoc. Within a minute, our main working directory `D:\Coding\obituary` was empty, including `.git`, our private docs repository `docs-internal/`, and the untracked `.claude/` directory. What ran the deletion was a string written into the release notes as an example: `find -delete`.

Our initial diagnosis was that the fix capable of blocking this command was already sitting in 1.4.2, just unreleased. A dozen minutes later, after checking the details, we realized that conclusion was wrong. Even if 1.4.2 had already been installed on the machine, it would have let that command through. The gap was not a missing deletion rule. Between the moment the delete guard inspects a command and the moment the command is actually parsed, a cross-shell handoff broke the quoting.

<figure>
  <img src="../../images/wiped-worktree/second-door.webp" alt="Concept illustration: in a dark space a figure stands before an archway, shining a torch through the first door; behind it is a second door, and an orange beam slants out of it where the figure cannot see" width="1024" height="572" loading="lazy" decoding="async">
  <figcaption>The delete guard stands at the first door and sees a piece of text; past the second door, another shell reads that text as a command to run.</figcaption>
</figure>

## How the incident happened

Obituary gives files a lifespan and buries them in a graveyard where they can later be resurrected, built primarily for AI agents. Its delete guard is a hook that runs before an agent executes a shell command. It denies literal `rm`, `rmdir`, `git clean`, and `Remove-Item` (including aliases) in command position, and suggests a resurrectable `obit` equivalent when one can be constructed. In our development setup, a coordinating agent (Claude Code) dispatches tasks and reviews changes while Codex workers perform the edits. On Windows, workers execute shell commands through bash inside WSL.

Trouble started that afternoon. Around 16:00, while replying to threads about AI agents deleting files, the coordinating agent saw a post on X showing Claude Code bypassing an `rm` block with `find -delete`. Our delete guard only recognized the four literal commands at the time and could not catch that pattern, so we sent a Codex worker into a separate worktree, `guard-find`, to patch it.

At 16:57, while submitting a test patch, the Bash examples inside the patch executed unexpectedly and emptied the worker's own worktree. There were no uncommitted tracked changes before the run, so tracked files were restored from Git. Without a prior list of ignored files, we could not verify whether any ignored files were affected. Both the main checkout and an adjacent worktree remained intact. This was the same class of failure as what happened that night—text inside a patch executing as commands—except it wiped a disposable copy, so it was reported to the owner as a minor incident. At 17:09, commit `b131063` landed on `main`. The delete guard began denying `find . -name x -delete`, `find . -exec rm -rf {} +`, `find . -print0 | xargs rm -rf`, and `xargs -0 rm`, while still allowing search-only calls like `find . -name '*.go'`. Because these commands target a set of files unknown ahead of time, the guard cannot safely rewrite them into an equivalent `obit` command; it denies without rewriting and tells the agent to list and verify the paths first, then bury them with `obit rm`.

When we started the 1.4.2 release that evening, two choices collided. First, the coordinating agent let the release worker operate directly inside the main checkout `D:\Coding\obituary` (`/mnt/d/Coding/obituary` in bash) instead of assigning a separate worktree. Second, step 2 of the release required writing the coordinator's release notes into `README.md`, `README.en.md`, the landing-page copy, and `CHANGELOG.md`. That copy contained Markdown backticks: `` `find -delete` ``, `` `find -exec rm` ``, `` `xargs rm` ``, `` `git clean` ``, `` `Remove-Item` `` and `` `obit rm` ``.

At 22:54:27, the worker first tried passing the patch through a quoted heredoc inside command substitution to Codex's apply-patch entry point, but omitted `cat`. It failed with `Invalid patch` and left all files untouched. The worker asked the coordinating agent for permission to retry. The coordinator approved and added that going forward, whenever a malformed command had not touched GitHub, mirrors, or local binaries, the worker should fix and retry directly without asking. At 22:56:19, it added `cat` and ran the command again:

```bash
codex --codex-run-as-apply-patch "$(cat <<'PATCH'
*** Begin Patch
*** Update File: README.md
@@
-- **怕 agent 一句 `rm -rf`。** ...
+- ... Bash 的 `find -delete`、`find -exec rm` 和 `xargs rm` 也会被拦下，但要先核对目标再用 `obit rm` 下葬。
...
*** Update File: CHANGELOG.md
@@
 - Fix: the delete guard now denies Bash `find -delete`, `find -exec rm`, and `xargs rm`, and explains how to list the targets first and bury them with `obit rm`.
...
*** End Patch
PATCH
)"
```

Under bash rules, the body of a quoted heredoc (`<<'PATCH'`) is literal text, and backticks inside it do not execute. Ten seconds later, the process was still running and had printed two rounds of the following stderr:

```text
fatal: clean.requireForce defaults to true and neither -i, -n, nor -f given; refusing to clean
/bin/bash: line 1: Remove-Item: command not found
/bin/bash: line 1: obit: command not found
```

After the worker interrupted the process, stderr produced `find: missing argument to '-exec'`, `rm: missing operand`, and `fatal: not a git repository (or any parent up to mount point /mnt)`, before the patch tool reported `README.md: No such file or directory`. At 22:57:02, `ls -la /mnt/d/Coding/obituary` returned only `total 0`. Remote `main` remained at `8a05e62`, untagged, with nothing published.

There is a hard boundary to our evidence. The tooling did not log the full arguments bash actually received after quoting broke, and the worker did not replay the command. What we observed directly is the stderr sequence: `git clean` refusing to run, `Remove-Item` and `obit` not found, `find -exec` missing an argument, and `rm` missing an operand. That sequence matches every backticked span in the patch executing as a command, with `find -delete` lacking a starting path and deleting recursively from the current directory. Because execution occurred inside WSL bash, nothing went to the Recycle Bin.

## Recovery relied on three unrelated copies

At 22:58, the coordinating agent stopped the worker and halted all writes to drive D: the deleted data might still be on the disk, and any write could overwrite it. Taking stock, all code was safe on GitHub, including `main` and every pushed branch. The exposed asset was our private documentation repo, `docs-internal/`: its last push to GitHub had been at 15:51 on 13 November, leaving six days of local-only edits.

While recommending Windows File Recovery (`winfr`) as a disk-level fallback, the coordinating agent restricted its own writes to drive C and pursued a second recovery path. Every time an agent read or wrote a file in `docs-internal/`, the full file text was recorded in the Claude or Codex session logs. Another worker had also run `git status` in the repository at 20:17 that evening, leaving its output in the logs. From that listing we confirmed that exactly 9 files had been added or modified since 13 November, matched them against session logs and GitHub Release text, and reconstructed every one. The owner chose to re-clone from GitHub. By 23:15 both repositories were restored, with the 9 recovered files placed back and pushed. What could not be recovered were a few older agent worktrees under `.claude/worktrees` and any unpushed local branches that might have existed.

The owner asked whether PowerShell should be removed. The deletion was carried out by bash inside WSL rather than PowerShell, so removing PowerShell would not have prevented it; Windows PowerShell 5.1 is a system component and cannot be uninstalled in any case. The risk was never a particular shell, but giving an automated process write permission in the directory holding the only copy.

## Where our first diagnosis went wrong

In its 23:11 report to the owner, the coordinating agent stated that once 1.4.2 shipped, the delete guard would block this kind of `find -delete`. At 23:25, while releasing 1.4.2 itself from a separate worktree and waiting for the build, it re-examined the failing command and retracted that claim.

The delete guard and the shell that executes the command do not see the same syntax tree. The hook runs before the command enters the next shell layer and inspects the raw string submitted by the worker. In that string, `` `find -delete` `` sits inside the body of a quoted heredoc (`<<'PATCH'`). Under bash syntax, that is plain text, and any compliant parser evaluates it as such. 1.4.2 does catch `find -delete` and `xargs rm` written directly in command position: before upgrading, we confirmed that 1.4.1 allowed `find ./probe -name x -delete`, and after upgrading to 1.4.2 the same command was denied. Placed inside a quoted heredoc, however, 1.4.2 still sees a safe command.

What turned text into executable commands was the handoff layer where Codex on Windows passes a command to WSL bash. Quoting broke during that transfer, effectively turning the heredoc unquoted and causing bash to evaluate the backticks as command substitution. That step happens after the hook, outside the delete guard's visibility.

The same check surfaced a configuration blind spot. On this machine, Claude Code's delete guard was running in `--notice-only` mode, showing graveyard hints without blocking deletions; enforcement was only turned on for Codex. With the owner's approval, we enabled enforcement for Claude Code via `obit init --guard-rm`.

## 1.4.3: Accounting for the handoff layer

Before changing the guard, we went back to a core rule in our architecture doc: deny broadly, rewrite narrowly. Deciding whether a command deletes files and deciding whether an exact replacement exists are separate problems. A denial executes nothing, so it can afford to be broad. An equivalent command must match literally, or it risks deleting unintended paths, so we only emit one inside a supported subset. Elsewhere in the guard, we prefer letting an ambiguous form pass rather than suggesting a command that could match too much; because the new rule only denies and never suggests a replacement, it can be stricter.

At 00:38 on 20 November, a Codex worker in the `guard-backticks` worktree submitted the initial implementation (`78a6bcf`). During review, the coordinating agent closed two gaps, committed the fixes at 00:54 (`2094645`), and released 1.4.3 at 01:06 (`b6cae61`). The rule, scoped to Codex on Windows, is called `EmbeddedDeletion` and rests on three design decisions.

First, it stops trusting quote and heredoc boundaries. Because quoting can fail during handoff, every backtick span and `$(…)` span in the raw command—whether inside single quotes, double quotes, a quoted heredoc, or comments—is extracted and checked. If any span parses as a deletion, the command is denied.

Second, every span is parsed under both Bash and PowerShell rules. The initial draft parsed only as Bash, which during review allowed single-quoted `'$(Remove-Item -Recurse x)'` through. Backticks execute only in Bash-like shells, but `$(…)` executes in both Bash and PowerShell. Once a command crosses the handoff boundary, the guard cannot know which shell will read the fragment, so it checks both interpretations.

Third, the rule runs before context resolution and does not depend on the working directory. The ordinary delete guard calls `deleteGuardContext` first to resolve the shell, working directory, and executable path so it can build an accurate `obit` replacement. Whenever any of those cannot be resolved—a WSL working directory, a PowerShell declaration without a Windows working directory, or an unavailable hook binary—the ordinary guard skips altogether. Because `EmbeddedDeletion` only denies and never rewrites, it needs none of that context. Placing it after `deleteGuardContext` would turn every unresolved context into a bypass.

Following project conventions, the deny reason is written as plain ASCII declarative statements. It tells the agent that Codex on Windows passes commands through another shell where quoting may fail, and that text containing backticks stays inert when written to a file with an editing tool and passed by file path.

<figure>
  <img src="../../images/wiped-worktree/embedded.webp" alt="Terminal screenshot: three commands fed to obit hook codex in the Windows build of 1.4.3. The patch command whose quoted heredoc holds plain text produces no output, meaning it is allowed; the same command with a pair of backticks added returns deny, with a reason explaining that Codex on Windows passes commands through another shell; $(Remove-Item -Recurse build) inside single quotes after echo also returns deny" width="1600" height="892" loading="lazy" decoding="async">
  <figcaption>1.4.3 on the real hook: in the same patch, one pair of backticks decides between allow and deny, and a single-quoted $(Remove-Item …) is denied too.</figcaption>
</figure>

The test suite covers PowerShell fragments, nested spans, quoted heredocs, Codex payloads carrying a POSIX working directory on Windows, and regression cases verifying that quoted heredocs remain literal under ordinary Bash parsing. We added mutation checks for each new predicate—scope, PowerShell parsing, and nested recursion—disabling one check at a time and requiring the suite to fail. All 5 new mutants were caught, and both the full mutation suite and CI passed. Feeding the previous night's exact command into the released Windows binary returned a denial, while ordinary commands passed through.

## Workflow rules, the sandbox we skipped, and the guard's ceiling

1.4.3 is only a backstop. The rules we wrote down that night carry the primary weight: workers stay out of the main checkout and run exclusively in separate worktrees; text containing backticks is written with file-editing tools and never passed through a shell; private docs are pushed immediately after edits. We also added a housekeeping rule to switch back to the user directory before launching long-running processes so they do not hold a lock on the working directory.

Between 23:30 and 00:02 the next morning, we also tested a harder boundary: Codex's built-in OS-level sandbox. In a temporary repository, it blocked writes and deletions outside the worktree with `Access is denied`. The side effects were immediate. Running `git commit` inside a separate worktree failed because Git writes metadata into the main repository's `.git` directory, which the sandbox blocked; network access was blocked as well. Switching directly would prevent every worker from committing and pushing. We tested a custom permission profile allowing the worker's worktree, its repository's `.git`, temporary directories, and networking, but `.git` writes and network access still failed. Getting it working would require changing Windows settings as administrator and moving workers off WSL onto another shell—high cost for an uncertain outcome—so we chose not to adopt it for now.

| Defense | Status | Against that night's command |
| --- | --- | --- |
| Workers operate only in separate worktrees | Active | Contains the damage; a repeat only wipes a disposable copy |
| Private docs pushed to GitHub | Pushed | Recoverable even if deleted locally |
| Text with backticks never passes through a shell | In rules | Prevents the failure at the source, but relies on worker compliance |
| 1.4.2 delete guard | Installed | Misses it; backticks sit inside a quoted heredoc and read as text |
| 1.4.3 `EmbeddedDeletion` | Shipped | Denies before execution |

The final row has a clear ceiling: the delete guard only understands deletion commands. If broken quoting next time triggers `git reset --hard` or truncates a file with `>`, the guard will not step in. Separate worktrees remain the main protection against those failure modes. If we later ask workers to run untrusted code, or hit another incident like this, paying the cost to get the OS sandbox working will be justified.

Our architecture document's threat model now includes a new row for commands mutating across shell handoffs. A delete guard only sees the text of a command, never how downstream shells re-interpret that text. Recovering six days of private docs depended on GitHub's code repository, the 13 November push, and the file contents and `git status` output that happened to live in session logs. Session logs are an accidental backup that nobody maintains, and we cannot rely on them next time.
