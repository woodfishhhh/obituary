/* OBIT_BRIDGE */

import { resolve as resolvePath } from "node:path";

function piWrittenPaths(agent, tool, input, cwd) {
  if (!["write", "edit"].includes(tool) || typeof input?.path !== "string" || !isAbsolute(cwd ?? "")) return [];
  // Pinned hosts: pi-mono 7d073328 utils/paths.ts; oh-my-pi 21f1188c
  // tools/{path-utils,plan-mode-guard}.ts. OMP preserves literal @name.
  // Neither exposes one common public resolver. Mirror only plain filesystem
  // names; skip home/URL/hashline/shell-drive spellings requiring host state.
  let target = input.path.replace(/[\u00A0\u2000-\u200A\u202F\u205F\u3000]/g, " ");
  if (agent === "pi" && target.startsWith("@")) target = target.slice(1);
  if (agent === "omp" && (target.startsWith("@") || /[\[\]#]/.test(target))) return [];
  if (!target || /^[~\\/]/.test(target) || /:/.test(target.replace(/^[A-Za-z]:[\\/]/, ""))) return [];
  return [agent === "omp" && isAbsolute(target) ? target : resolvePath(cwd, target)];
}

// Pi and OMP load the same module. OMP documents pi.pi (package exports);
// the callback protocols otherwise differ only at result context delivery.
export function registerPi(pi, options = {}) {
  const agent = pi.pi ? "omp" : "pi";
  const bridge = createBridge(agent, options);
  const base = (ctx) => ({ session: ctx?.sessionManager?.getSessionId?.(), cwd: ctx?.cwd });

  pi.on("tool_call", safeCallback(async (event, ctx) => {
    if (!bridge.guard || !["bash", "powershell"].includes(event?.toolName) || typeof event?.input?.command !== "string") return;
    const reply = await bridge.call({ ...base(ctx), event: "PreToolUse", host_event: "tool_call",
      tool: event.toolName, command: event.input.command, shell: bridge.shell,
      local: localInput(bridge, event.input) });
    if (reply.deny) return { block: true, reason: reply.reason };
  }));

  pi.on("tool_result", safeCallback(async (event, ctx) => {
    if (event?.isError !== false || !localInput(bridge, event.input)) return;
    const paths = piWrittenPaths(agent, event.toolName, event.input, ctx?.cwd);
    if (!paths.length) return;
    const reply = await bridge.call({ ...base(ctx), event: "PostToolUse", host_event: "tool_result",
      tool: event.toolName, paths, success: true, context_delivery: true });
    if (!reply.context) return;
    if (agent === "omp") return { additionalContext: reply.context };
    return { content: [...(Array.isArray(event.content) ? event.content : []), { type: "text", text: reply.context }] };
  }));

  pi.on("session_start", safeCallback(async (_event, ctx) => {
    if (!bridge.local) return;
    const delivery = typeof pi.sendMessage === "function";
    const reply = await bridge.call({ ...base(ctx), event: "SessionStart", host_event: "session_start", context_delivery: delivery });
    if (reply.context && delivery) await pi.sendMessage({ customType: "obituary", content: reply.context, display: false }, { triggerTurn: false });
  }));
  for (const host of agent === "omp" ? ["agent_end", "session_stop"] : ["agent_end", "agent_settled"]) {
    pi.on(host, safeCallback(async (_event, ctx) => {
      if (bridge.local) await bridge.call({ ...base(ctx), event: "Stop", host_event: host });
    }));
  }
  pi.on("session_shutdown", safeCallback(async (_event, ctx) => {
    if (bridge.local) await bridge.call({ ...base(ctx), event: "SessionEnd", host_event: "session_shutdown" });
  }));
}

export default function (pi) { registerPi(pi); }
