import { execFile } from "node:child_process";
import { readFileSync } from "node:fs";
import { isAbsolute } from "node:path";

// Protocol conversion only. Node's execFile supplies argv/stdin without a shell.
export function createBridge(agent, options = {}) {
  const env = options.env ?? process.env;
  let settings = options.settings;
  if (!settings) {
    try {
      const name = agent === "openclaw" ? "./runtime.json" : `./obituary/${agent}.json`;
      settings = JSON.parse(readFileSync(new URL(name, import.meta.url), "utf8"));
    } catch { settings = {}; }
  }
  const prefix = `OBIT_${agent.toUpperCase()}_`;
  // Host tool names do not prove execution locality: extensions can replace
  // built-ins with SSH/container backends. Only explicit operator attestation
  // enables observation/guard; model-supplied tool arguments cannot enable it.
  const local = env[prefix + "LOCAL"] === "1";
  const shell = ["bash", "powershell"].includes(env[prefix + "SHELL"]) ? env[prefix + "SHELL"] : "";
  const invoke = options.execFile ?? execFile;
  return {
    local,
    shell,
    guard: settings.guardRM === true,
    async call(fields) {
      try {
        if (!isAbsolute(settings.executable ?? "")) return {};
        const payload = JSON.stringify({ version: 1, agent, local, context_delivery: false, ...fields });
        if (Buffer.byteLength(payload) > 4 * 1024 * 1024) return {};
        return await new Promise((resolve) => {
          const child = invoke(settings.executable, ["hook", agent], {
            encoding: "utf8", shell: false, windowsHide: true,
            timeout: 1200, maxBuffer: 64 * 1024,
          }, (error, stdout) => {
            if (error) return resolve({});
            try {
              const reply = JSON.parse(stdout);
              if (reply?.version !== 1) return resolve({});
              resolve({
                deny: reply.deny === true && typeof reply.reason === "string" && reply.reason.length > 0,
                reason: typeof reply.reason === "string" ? reply.reason : "",
                context: typeof reply.context === "string" ? [...reply.context].slice(0, 2000).join("") : "",
              });
            } catch { resolve({}); }
          });
          child.stdin.on("error", () => resolve({}));
          child.stdin.end(payload);
        });
      } catch { return {}; }
    },
  };
}

function localInput(bridge, input) {
  return bridge.local && !input?.remote && !input?.sandbox && !input?.container &&
    !["ssh", "remote", "container", "docker", "sandbox", "node"].includes(input?.host) &&
    !["ssh", "remote", "container", "docker", "sandbox"].includes(input?.backend);
}

function writtenPaths(tool, input) {
  if (!["write", "edit"].includes(tool)) return [];
  const path = input?.filePath ?? input?.path ?? input?.file_path;
  return typeof path === "string" && path.length > 0 ? [path] : [];
}

function safeCallback(callback) {
  return async (...args) => {
    try { return await callback(...args); } catch { return undefined; }
  };
}
