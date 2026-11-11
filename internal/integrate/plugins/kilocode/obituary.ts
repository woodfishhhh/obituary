// Kilo PluginModule API at b6a49d01; intentional tool exceptions propagate.
// Kilo-Org/kilocode packages/plugin/src/index.ts defines the host contract.
// execFile keeps argv independent of shell parsing.
import { execFile } from "node:child_process";
import path from "node:path";
const settings = __OBIT_SETTINGS__;

function call(request) {
  return new Promise((resolve) => {
    try {
      const payload = JSON.stringify({ version: 1, agent: "kilocode", ...request });
      if (Buffer.byteLength(payload) > 4 * 1024 * 1024) return resolve({});
      const child = execFile(settings.executable, ["hook", "kilocode"], {
        shell: false, windowsHide: true, timeout: 1500, maxBuffer: 16384, encoding: "utf8",
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

export default {
  id: "obituary",
  async server(ctx) {
    const cwd = typeof ctx.directory === "string" ? ctx.directory : "";
    // Versioned Kilo clients vary in shell defaults and remote Location support.
    // Only an explicit host-environment assertion proves this tool runtime.
    const shell = process.env.OBIT_KILOCODE_LOCAL_SHELL ?? "";
    const local = path.isAbsolute(cwd) && (shell === "bash" || shell === "powershell");
    function event(name, host, session, fields = {}) {
      return { event: name, host_event: host, session, cwd, shell, local,
        context_delivery: false, ...fields };
    }
    const hooks = {
      async "experimental.chat.system.transform"(input, output) {
        if (!local || typeof input.sessionID !== "string" || !Array.isArray(output.system)) return;
        const reply = await call(event("Context", "experimental.chat.system.transform", input.sessionID, { context_delivery: true }));
        if (typeof reply.context === "string" && reply.context) output.system.push(reply.context);
      },
      async "tool.execute.before"(input, output) {
        let response;
        try {
          if (!settings.guard || input.tool !== "bash" || typeof output.args?.command !== "string") return;
          response = await call(event("PreToolUse", "tool.execute.before", input.sessionID, {
            tool: input.tool, command: output.args.command,
            cwd: typeof output.args.workdir === "string" ? path.resolve(cwd, output.args.workdir) : cwd,
          }));
        } catch { return; }
        // A tool error is the V1 host's veto channel. Never catch this exception.
        if (response.deny === true && typeof response.reason === "string") throw new Error(response.reason);
      },
      async "tool.execute.after"(input, output) {
        if (!local || output.metadata?.error || output.metadata?.isError) return;
        let paths = [];
        if (input.tool === "write" || input.tool === "edit") {
          if (typeof input.args?.filePath === "string") paths = [input.args.filePath];
        } else if (input.tool === "apply_patch" && typeof input.args?.patchText === "string") {
          paths = input.args.patchText.split(/\r?\n/).flatMap((line) => {
            const match = /^\*\*\* (?:Add File|Update File|Move to): (.+)$/.exec(line);
            return match ? [match[1]] : [];
          });
        }
        if (paths.length) await call(event("PostToolUse", "tool.execute.after", input.sessionID, {
          tool: input.tool, paths, success: true,
        }));
      },
      async event({ event: input }) {
        const names = { "session.created": "SessionStart", "session.idle": "Stop", "session.deleted": "SessionEnd" };
        const name = names[input.type];
        const session = input.properties?.sessionID ?? input.properties?.info?.id;
        if (name && typeof session === "string") await call(event(name, input.type, session));
      },
    };
    for (const name of ["tool.execute.after", "event", "experimental.chat.system.transform"]) {
      const handler = hooks[name];
      hooks[name] = async (...args) => {
        try { return await handler(...args); } catch { return undefined; }
      };
    }
    return hooks;
  },
};
