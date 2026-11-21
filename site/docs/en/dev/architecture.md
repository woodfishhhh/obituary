# Architecture

Consider marking a file: the CLI parses its path and lifespan, the application opens the service, and the core records its identity and deadline. Success at this point means the file is tracked, not that it has moved. A sweep or an explicit burial performs the move.

## Where decisions live

| Entry point or package | Responsibility |
| --- | --- |
| CLI, window, MCP | Translate input into service calls and results into exit codes, UI state or protocol replies. None moves files itself. |
| Host hooks | Translate host payloads into `internal/hook.Event`. A runner takes slow work off the tool-call path. |
| `internal/integrate` | Plan changes to host configuration, then check the original content before applying the plan so concurrent edits survive. |
| `internal/app`, `obitd` | Assemble platform services and wake the background worker. The core still decides what is eligible. |
| `internal/core`, `internal/core/ledger` | Check safety rules and identity; coordinate moves, ledger revisions, recovery and audit records. |

## After the deadline

A sweep claims a due entry, locates the object again and checks its identity. Burial creates a grave exclusively and writes its manifest before moving the contents. If the move succeeds but the ledger update is interrupted, recovery compares the manifest to the actual locations. A revision check prevents an old sweep snapshot from overwriting a new extension.

The ledger describes current state; the audit log describes completed operations. If an audit write fails, the outbox can deliver it later. Replaying the file move to repair the log would be incorrect. See the [safety model](./safety) for failure handling.
