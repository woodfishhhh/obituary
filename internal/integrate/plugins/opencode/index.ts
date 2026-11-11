/* OBIT_BRIDGE */

// The dual object export supports the host's V1/V2 plugin APIs (>=1.18.29).
// V2 Promise hooks accept the schema Tool.Error exported by promise/tool.
export function createOpenCode(options = {}) {
  const bridge = createBridge("opencode", options);
  async function before(tool, input, session, cwd, host) {
    if (!bridge.guard || !["bash", "shell"].includes(tool) || typeof input?.command !== "string") return {};
    return bridge.call({ event: "PreToolUse", host_event: host, tool, session, cwd,
      command: input.command, shell: bridge.shell, local: localInput(bridge, input) });
  }
  async function after(tool, input, session, cwd, host) {
    const paths = writtenPaths(tool, input);
    if (!paths.length || !localInput(bridge, input)) return {};
    return bridge.call({ event: "PostToolUse", host_event: host, tool, session, cwd,
      success: true, paths, context_delivery: true });
  }
  const lifecycle = safeCallback(async (name, session, cwd, host) => {
    if (bridge.local) await bridge.call({ event: name, host_event: host, session, cwd });
  });
  return {
    id: "obituary",
    async setup(ctx) {
      let ToolError;
      try { ToolError = options.ToolError ?? (await import("@opencode/plugin/promise/tool")).Error; } catch { /* Unknown V2 SDK: guard stays unavailable. */ }
      const cwd = ctx?.location?.directory;
      await ctx.tool.hook("execute.before", async (event) => {
        let reply;
        try { reply = await before(event.tool, event.input, event.sessionID, cwd, "execute.before"); } catch { return; }
        // Intentional denial must escape the fail-open transport catch.
        if (reply.deny && ToolError) throw new ToolError({ message: reply.reason });
      });
      await ctx.tool.hook("execute.after", safeCallback(async (event) => {
        if (event.status !== "completed" || !event.result) return;
        const reply = await after(event.tool, event.input, event.sessionID, cwd, "execute.after");
        if (reply.context) event.result.content = [...(event.result.content ?? []), { type: "text", text: reply.context }];
      }));
      const seen = new Set();
      await ctx.session.hook("prompt", safeCallback(async (event) => {
        if (seen.has(event.sessionID)) return;
        seen.add(event.sessionID);
        await lifecycle("SessionStart", event.sessionID, cwd, "prompt");
      }));
      await ctx.session.hook("experimental.idle", (event) => lifecycle("Stop", event.sessionID, cwd, "experimental.idle"));
    },
    async server(ctx = {}) {
      const cwd = ctx.directory;
      return {
        "tool.execute.before": async (event, output) => {
          let reply;
          try { reply = await before(event.tool, output.args, event.sessionID, cwd, "tool.execute.before"); } catch { return; }
          if (reply.deny) throw new Error(reply.reason);
        },
        "tool.execute.after": safeCallback(async (event, output) => {
          if (typeof output?.output !== "string" || output.error || output.metadata?.error) return;
          const reply = await after(event.tool, event.args, event.sessionID, cwd, "tool.execute.after");
          if (reply.context) output.output += "\n" + reply.context;
        }),
        event: safeCallback(async ({ event }) => {
          const session = event.properties?.info?.id ?? event.properties?.sessionID;
          const names = { "session.created": "SessionStart", "session.idle": "Stop", "session.deleted": "SessionEnd" };
          if (names[event.type]) await lifecycle(names[event.type], session, cwd, event.type);
        }),
      };
    },
  };
}

export default createOpenCode();
