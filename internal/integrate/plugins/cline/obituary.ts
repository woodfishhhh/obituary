// Cline SDK/CLI/Kanban AgentPlugin (94eecbbe); VS Code/JetBrains excluded.
// docs.cline.bot/sdk/plugins; execFile keeps argv independent of shell parsing.
import { execFile } from "node:child_process";
import path from "node:path";
const settings = __OBIT_SETTINGS__;

function call(request, timeout = 1500) {
  return new Promise((resolve) => {
    try {
      const payload = JSON.stringify({ version: 1, agent: "cline", ...request });
      if (Buffer.byteLength(payload) > 4 * 1024 * 1024) return resolve({});
      const child = execFile(settings.executable, ["hook", "cline"], {
        shell: false, windowsHide: true, timeout, maxBuffer: 16384, encoding: "utf8",
      }, (error, stdout) => {
        try {
          const response = error ? {} : JSON.parse(stdout);
          resolve(response?.version === 1 ? response : {});
        } catch { resolve({}); }
      });
      child.stdin.on("error", () => {});
      child.stdin.end(payload);
    } catch { resolve({}); }
  });
}

// SDK executors can be remote. This process-level assertion is supplied by the
// user after checking the executor; tool input cannot assert local execution.
function runtime(cwd) {
  const shell = process.env.OBIT_CLINE_LOCAL_SHELL;
  return { cwd, shell: shell ?? "", local: path.isAbsolute(cwd ?? "") &&
    (shell === "bash" || shell === "powershell") };
}
function patchPaths(patch) {
  if (typeof patch !== "string") return [];
  return patch.split(/\r?\n/).flatMap((line) => {
    const match = /^\*\*\* (?:Add File|Update File|Move to): (.+)$/.exec(line);
    return match ? [match[1]] : [];
  });
}
let cwd = "", session = "", started = false;
function event(name, host, fields = {}) {
  return { event: name, host_event: host, session, ...runtime(cwd),
    context_delivery: name === "SessionStart" || name === "PostToolUse", ...fields };
}
function context(response) {
  return typeof response.context === "string" && response.context ?
    { appendContext: response.context } : undefined;
}

const plugin = {
  name: "obituary",
  manifest: { capabilities: ["hooks"] },
  setup(_api, ctx) {
    cwd = typeof ctx.cwd === "string" ? ctx.cwd : "";
    session = typeof ctx.session?.sessionId === "string" ? ctx.session.sessionId : "";
    started = false;
    ctx.onDispose?.(async () => { await call(event("SessionEnd", "onDispose")); });
  },
  hooks: {
    async beforeRun() {
      if (started || !session || !runtime(cwd).local) return;
      started = true;
      return context(await call(event("SessionStart", "beforeRun")));
    },
    async beforeTool(ctx) {
      const tool = ctx.tool?.name ?? ctx.toolCall?.name;
      if (!settings.guard || tool !== "run_commands") return;
      const commands = ctx.input?.commands;
      if (!Array.isArray(commands)) return;
      const deadline = Date.now() + 1800;
      for (const command of commands) {
        if (typeof command !== "string" || Date.now() >= deadline) continue;
        const response = await call(event("PreToolUse", "beforeTool", {
          tool, command, context_delivery: false,
        }), Math.max(1, Math.min(1000, deadline - Date.now())));
        // Intentional denial stays outside the transport error handler.
        if (response.deny === true && typeof response.reason === "string") {
          return { skip: true, reason: response.reason };
        }
      }
    },
    async afterTool(ctx) {
      const tool = ctx.tool?.name ?? ctx.toolCall?.name;
      if (!runtime(cwd).local || ctx.result?.success !== true || ctx.result?.error) return;
      const paths = tool === "editor" || tool === "edit_file" ?
        (typeof ctx.input?.path === "string" ? [ctx.input.path] : []) :
        tool === "apply_patch" ? patchPaths(typeof ctx.input === "string" ? ctx.input : ctx.input?.input) : [];
      if (!paths.length) return;
      return context(await call(event("PostToolUse", "afterTool", {
        tool, paths, success: true,
      })));
    },
    async afterRun() { await call(event("Stop", "afterRun")); },
  },
};
// SDK hosts may use hookErrorMode=throw. Extraction failures also fail open.
for (const [name, handler] of Object.entries(plugin.hooks)) {
  plugin.hooks[name] = async (...args) => {
    try { return await handler(...args); } catch { return undefined; }
  };
}
export default plugin;
