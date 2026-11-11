# Grave notices

Burying first only works if a burial can be remembered. When an agent's tool call touches a path that is buried and still restorable (or something inside a buried directory), Obituary tells it:

- the grave number;
- when it was buried;
- how long it stays restorable;
- `obit resurrect <id>`.

## It only informs

- The tool call is never blocked or changed.
- Each grave is mentioned once per session.
- Claude Code and Codex hear it before the tool runs; other agents at the point listed in `obit init --show`.
- The rest of the time it stays out of the way: an agent starts work knowing only that Obituary is there, without reading a list first.

Existing installations get grave notices by running `obit init` again.

## Session summary (optional)

To save the agent's tokens, a session starts with only the proposals that need your decision. Turn on "Session summary" in the small window's Settings, or set:

```toml
[hooks]
session_summary = true
```

The agent then also hears this project's items due within 24 hours and its recent restorable burials (`obit mktemp` scratch directories excluded).

## Look for yourself

```powershell
obit graveyard           # what is in the graveyard and when it will be purged
obit graveyard --here    # this folder only
obit resurrect <id>      # put it back; if the name is taken, it comes back renamed, never overwriting
obit resurrect --last    # undo the last deletion: everything one obit bury / obit rm buried comes back
```
