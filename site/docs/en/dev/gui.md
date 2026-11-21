# Desktop window

The window does not have its own ledger. `cmd/obit-gui` exposes the `internal/gui` API through Wails; React consumes it through `frontend/src/api.ts`. Browser development uses an in-memory mock of that interface. A preview can verify layout and interaction, not that a real file was restored.

`App.tsx` reads a summary and shares a refresh signal with the current page. Most looks show Upcoming, Graveyard, Decisions, Settings and Logs. The timeline look combines the first two. `catalog.ts` lazy-loads eight shipping looks; experimental looks stay in the playground. When changing an operation, check its Go binding, frontend type and mock result together.

| Target | Build and acceptance |
| --- | --- |
| Windows amd64 | Build with `desktop,production` and the Windows GUI subsystem. Verify native window and tray behavior on Windows. |
| Ubuntu 24.04 amd64 | Requires CGO and GTK 3 and WebKitGTK 4.1 development packages. `linux-desktop.yml` exercises the package under a private Xvfb display and session bus. |
| macOS | `macos-desktop.yml` tests native bindings on a macOS runner and builds the universal app and DMG. Ad-hoc signing is not notarization. |

The frontend lives in `cmd/obit-gui/frontend`. After editing it, commit the rebuilt `dist/` embedded by Go. See [Build and test](./build-test) for the other platform checks.
