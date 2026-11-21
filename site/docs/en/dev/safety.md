# Safety model

A path tells Obituary where to look; identity tells it whether it found the same object. A file can be renamed before expiry and a different file can take its old name. The replacement does not inherit the deadline.

When changing code that moves, buries or deletes files, first write a failing test that checks the file that should survive is still there. Then fix the code. Temporarily restore the faulty implementation, keeping it compilable, and confirm the test fails before reinstating the fix.

| Stage | Required invariant |
| --- | --- |
| Registration and lookup | The guard checks protected paths. Automatic registration also checks Git state; uncertainty is not permission to proceed. |
| Burial | `FS.MovePinned` rechecks identity for the move. If the platform cannot meet the safety conditions, preserve the original. Do not fall back to deletion. |
| Restoration | Verify grave ownership, manifest and destination parent before restoring with `MovePinnedInto`. Choose another name if the original is taken; never overwrite. |
| Purging | Only `Purge`, requested explicitly or after retention expires, may permanently remove user content in a verified grave. Recycle Bin failure is not permission to purge. |

`ErrNotFound` means absence was established. `ErrUnsafe` means refusal. Access errors and sharing conflicts do not mean a file disappeared: keep the record for retry. An audit delivery failure must not undo the recorded outcome of an operation. See the [filesystem contract](https://github.com/woodfishhhh/obituary/blob/main/docs/architecture.md) for platform-specific guarantees and limits.
