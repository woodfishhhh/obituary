---
title: 'macOS and Linux: no rename by handle'
description: 'On Windows a burial checks identity and renames on the same handle, while rename on macOS and Linux knows only a parent directory and a name. On 10 November we decided not to relax that promise. Two days later we switched to moving first, then verifying, and putting the file back if it does not match, moved the whole test suite onto CI for both platforms the same day, and released 1.1.0 on 13 November.'
developmentDate: '2026-11-12'
translationKey: macos-linux-move
sequence: 13
draft: false
---

On Windows a burial does one very specific thing. It opens the file, checks its identity on that handle and renames it on the same handle, so there is no chance for the file to be swapped between the check and the rename. macOS and Linux have no such system call, since `rename`, `renameat2` and `renameatx_np` all take a parent directory and a name. If an editor swaps in a new file after the check and before the rename, the new file is the one that gets moved.

## Not relaxing it, at first

Our decision on 10 November was not to relax the promise. `core.FS` demands that a burial check and then move atomically, and on a platform that cannot do that, any operation that needs it returns unsupported, the entry is kept and nothing is buried. The alternative was to hard-link the verified inode into the graveyard through `/proc/self/fd`, then move it by name, check it and move it back if it was wrong. For files that shrinks the window a lot, but on a conflict the file that had been swapped in may sit in a quarantine directory until someone puts it back, and for directories it cannot be done at all. We did not take it.

The cost was written down plainly. On macOS and Linux for the time being you could only register, look and use hooks, while burying, resurrecting and the Recycle Bin returned that the platform does not support them yet, and expired items were not cleaned up.

## Move first, then verify

Two days later the line moved. Without burial and resurrection the core feature was unusable on those two platforms, and many users are on a Mac. On 12 November we asked for macOS and Linux to be supported quickly, and the way to do it was to move first, then verify, and put the file back if it does not match.

In detail it goes like this. The parent directories of source and target are pinned, and the object is moved with a rename that never overwrites, which is `RENAME_NOREPLACE` with `renameat2` on Linux and `RENAME_EXCL` with `renameatx_np` on macOS. The source object is separately pinned as a file descriptor with `openat`, `O_PATH` on Linux and `O_EVTONLY` on macOS, and it stays open for the whole operation, so the inode cannot be released and reused during the move. As soon as the move is done, the device number, inode, type and link count are checked at the new location. If they do not match, the same non-overwriting rename puts it back and a retryable error is returned. If it cannot be put back because the original place is occupied, the object stays in the graveyard, its record is marked as an identity mismatch and it is never purged, and `obit graveyard` and `obit doctor` show where it is and how to deal with it by hand.

Some file systems do not support a non-overwriting rename at all. What follows is a small experiment in Ubuntu under WSL. The same `RENAME_NOREPLACE` succeeds on ext4 when the target does not exist and reports that the file exists when it does. On `/mnt/c`, where the Windows disk is mounted through 9p, it returns invalid argument when the target is missing. On a file system like that Obituary returns unsupported and does not fall back to a plain `rename`.

<figure>
  <img src="../../images/macos-linux-move/renameat2.webp" alt="Terminal screenshot: renameat2 with RENAME_NOREPLACE called once on ext4 and once on the 9p file system under WSL, on ext4 it reports file exists when the target exists and succeeds when it does not, on the 9p mount /mnt/c it returns invalid argument when the target is missing" width="1600" height="372" loading="lazy" decoding="async">
  <figcaption>The real result of the same non-overwriting rename on two file systems. ext4 supports it and 9p returns invalid argument, so Obituary refuses to bury in such places.</figcaption>
</figure>

What is weaker than on Windows is also in the decision. If an editor swaps in a new file exactly between the check and the rename, that new file is briefly moved away and put back, or left in the graveyard for a person to handle, and it is not deleted.

## A different file at the same path

Identity checking has one more problem on POSIX that Windows does not have: inodes get reused. On Linux, when a file is deleted and a new one is created at the same path, the new file often gets the same inode, and an entry registered days earlier would take the newcomer for the old file. The fix on the evening of 12 November added a creation token to the upper 8 bytes of the file ID. On Linux it is a hash of the export file handle, which carries the inode's generation, combined with the `statx` creation time, and on macOS it is the creation time. A file system that cannot supply such a token, for example overlayfs without `nfs_export`, refuses lifespans, while an immediate `bury` still works because it pins the file descriptor. Entries in an old ledger that hold only an inode are kept at expiry, and `list` and `doctor` ask for a fresh `obit mark`.

Below, the Linux build reproduces this on ext4 under WSL. After the file is deleted and recreated, the new file gets exactly the same inode number, the expiry sweep does not bury it, and the entry is kept and retried later.

<figure>
  <img src="../../images/macos-linux-move/inode-reuse.webp" alt="Terminal screenshot: on Linux a.txt gets a 6-second lifespan, is deleted and recreated, both stat calls show inode 703107, the expiry sweep reports that the entry is kept and retried later, a.txt still contains new and the graveyard is empty" width="1600" height="587" loading="lazy" decoding="async">
  <figcaption>The Linux build really run on ext4 under WSL. The file that was deleted and recreated got the same inode, and Obituary did not take the new file for the old one and bury it.</figcaption>
</figure>

## The whole suite on both platforms

That evening at 8:58 the whole test suite ran on Linux and macOS. Two CI jobs run `go test ./...` plus native safety mutations, and today the Linux job also runs with `-race`. The fixtures were rewritten so that drive letters, junctions, the fake Docker and the temporary directories work on both sides. Behavior that exists only on Windows, such as alternate data streams, extended paths, the registry, Explorer and shortcuts, is skipped on POSIX with a stated reason, and where POSIX lacks a capability, such as finding an inode across a whole volume, the test asserts the conservative outcome.

Porting the tests turned up a few product problems, with Windows behavior unchanged. The delete guard treated every POSIX working directory as WSL interop, so deletions in native Linux and macOS shells were not guarded. Uninstall now goes through `obitd` and skips the Run key, Explorer and the context menu. Registration and observation now check known tracked paths and no longer fail because a whole-volume lookup is unsupported. Names containing a colon can be resurrected. At 10:15 that night the first round of CI fixes covered a batch of macOS habits: the case-insensitive APFS, `~/Library/Application Support`, and temporary directories that resolve under `/var`.

On 13 November the work was merged at 12:02 and 1.1.0 was published at 12:17. On macOS expiry is checked by launchd and on Linux by a systemd user timer. macOS can install through a Homebrew cask, both platforms can use the install script that verifies SHA-256 and installs into `~/.local/bin`, and the delete guard catches `rm -rf` in the terminals of both.
