# Add an agent

Start with what the host actually exposes. A post-write hook can report a completed write; only a pre-tool hook can intervene before a deletion. MCP tools require a model invocation, and a Skill is guidance. Neither provides a missing host event. Record these capabilities separately in the coverage matrix.

1. Add a thin adapter in `internal/hook` to translate real payloads into the common `Event`. Use redacted fixtures for missing fields, paths and event types. Delegate slow work to the runner.
2. Describe the installation target in `internal/integrate` and resolve configuration through `internal/userdirs`. Test existing configuration, repeated installation and removal: uninstall only what Obituary owns. Extend `doctor` for new installation steps.
3. Check discovery of MCP, Skills and host plugins independently. Decisions reserved for the user, such as trusting project rules, should not become MCP tools.
4. Verify the generated `docs/agents.md` matrix and update the guides driven by `site/docs/.vitepress/agents` and the landing data. Exercise installation and one event in the real host: a successful write does not prove a hook observed it.
