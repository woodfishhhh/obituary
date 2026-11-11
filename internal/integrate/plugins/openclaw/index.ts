/* OBIT_BRIDGE */

// OpenClaw tool hook results use block/blockReason. Observation callbacks defer
// notices until the host grants conversation access and prompt modification.
export function registerOpenClaw(api, options = {}) {
  const bridge = createBridge("openclaw", options);
  const env = options.env ?? process.env;
  const base = (event, ctx) => ({ session: ctx?.sessionId ?? ctx?.sessionKey ?? event?.sessionId,
    cwd: ctx?.workspaceDir ?? event?.params?.workdir ?? env.OBIT_OPENCLAW_CWD });
  api.on("before_tool_call", safeCallback(async (event, ctx) => {
    if (!bridge.guard || event?.toolName !== "exec" || typeof event?.params?.command !== "string") return;
    if (event.toolKind === "code_mode_exec" || ctx?.toolKind === "code_mode_exec" ||
      ["javascript", "typescript"].includes(event.toolInputKind ?? ctx?.toolInputKind)) return;
    const reply = await bridge.call({ ...base(event, ctx), event: "PreToolUse", host_event: "before_tool_call",
      tool: event.toolName, command: event.params.command, shell: bridge.shell,
      local: localInput(bridge, event.params) });
    if (reply.deny) return { block: true, blockReason: reply.reason };
  }));
  api.on("after_tool_call", safeCallback(async (event, ctx) => {
    if (event?.error || event?.result === undefined || event.result?.isError || !localInput(bridge, event.params)) return;
    const paths = writtenPaths(event.toolName, event.params);
    if (paths.length) await bridge.call({ ...base(event, ctx), event: "PostToolUse", host_event: "after_tool_call",
      tool: event.toolName, success: true, paths });
  }));
  api.on("before_prompt_build", safeCallback(async (event, ctx) => {
    const permissions = api.config?.plugins?.entries?.obituary?.hooks;
    if (!bridge.local || permissions?.allowConversationAccess !== true || permissions.allowPromptModification === false) return;
    const reply = await bridge.call({ ...base(event, ctx), event: "Context", host_event: "before_prompt_build", context_delivery: true });
    if (reply.context) return { prependContext: reply.context };
  }));
  for (const [host, neutral] of [["session_start", "SessionStart"], ["agent_end", "Stop"], ["session_end", "SessionEnd"]]) {
    api.on(host, safeCallback(async (event, ctx) => {
      if (bridge.local) await bridge.call({ ...base(event, ctx), event: neutral, host_event: host });
    }));
  }
}

export default function (api) { registerOpenClaw(api); }
