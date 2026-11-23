# Developer guide

Start with [AGENTS.md](https://github.com/woodfishhhh/obituary/blob/main/AGENTS.md) for repository conventions and the checks your change needs. If it crosses package boundaries, read the [architecture contract](https://github.com/woodfishhhh/obituary/blob/main/docs/architecture.md). See [CONTRIBUTING](https://github.com/woodfishhhh/obituary/blob/main/CONTRIBUTING.md) before reporting a problem or submitting a patch. For installation and everyday commands, use the [user guide](../).

| What are you changing? | Where to start |
| --- | --- |
| How a command changes filesystem state | [Architecture](./architecture) · [Safety model](./safety) |
| A package or its tests | [Repository layout](./repository) · [Build and test](./build-test) |
| The desktop window or a host adapter | [Desktop window](./gui) · [Add an agent](./agent-integration) |
| Translations, documentation or release artifacts | [Internationalization](./i18n) · [Writing docs and blog posts](./docs-and-blog) · [Releases and mirrors](./release) |

The CLI and daemon target Windows, macOS and Linux. Desktop packages cover Windows amd64, Ubuntu 24.04 amd64 and a universal macOS app. Explorer integration is Windows-only. System Trash is available on Windows and Linux.
