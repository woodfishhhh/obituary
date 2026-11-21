# Build and test

Use Go 1.26.4 or later as specified by `go.mod`. Run these commands from the repository root. Building multiple packages without an output path checks compilation without installing anything or starting a service.

```bash
go build ./cmd/obit ./cmd/obitd
go test -short ./internal/core/lifespan
```

To produce Windows binaries, build the two programs separately. The daemon uses the GUI subsystem so scheduled runs do not open a console.

```bash
go build -o bin/obit.exe ./cmd/obit
go build -ldflags=-H=windowsgui -o bin/obitd.exe ./cmd/obitd
```

The Linux and macOS CLI builds need no desktop libraries. In Bash, `GOOS=darwin GOARCH=arm64 go build ./cmd/obit ./cmd/obitd` checks compilation for an Apple silicon target. It does not test notifications, the menu bar or a first launch. See [Desktop window](./gui) for native desktop prerequisites.

## Choose the scope

Run the full tests for the affected package and short tests for its callers locally. Leave the whole suite to CI. For a change that moves or deletes data, write a failing test first. After the fix, restore a compilable faulty implementation temporarily to prove the test detects it. CI runs sharded MCP and delete-guard mutations on a schedule or on demand.

The website builds independently. In `site/`, use the workspace's pinned pnpm version and check both deployment targets:

```bash
pnpm install --frozen-lockfile
pnpm build
pnpm build:cn
```
