# Repository layout

| Path | What belongs here |
| --- | --- |
| `cmd/` | Wire up the CLI, daemon and desktop app. Implement commands in `internal/cli/` and register them in the entry point. |
| `internal/` | `app` opens services, `core` owns lifecycles, and `gui` exposes desktop bindings. Resolve user directories through `userdirs`. |
| `site/` | The React homepage lives in `landing`, the VitePress guides in `docs`, and assembly scripts in `scripts`. |
| `docs/` | Architecture contract, manuals, agent coverage and illustrations. Start at `docs/README.md`. |
| `scripts/`, `packaging/` | Build and acceptance scripts, installers and desktop packaging materials. |
| `.github/workflows/`, `release/` | CI and deployment definitions, and public notes for individual releases. |

Pair tests with their implementation: behavior in `bury.go` belongs in `bury_test.go`. Put cross-package protocol and subprocess scenarios in `internal/e2e`. New test packages follow the neighboring `TestMain` so `internal/testenv` isolates the ledger, user directories and platform integration state.
