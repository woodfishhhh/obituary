/* OBIT_BRIDGE */

import { posix as posixPath } from "node:path";

// The official Claude bridge drops isError and canonical value from PostToolUse.
// tools/result supplies both after the complete pipeline; keep this observer
// separate and leave guard/context/lifecycle transport with the official bridge.
export function observeDeepSeek(ctx, config, options = {}) {
  const env = options.env ?? process.env;
  const platform = options.platform ?? process.platform;
  const bridge = createBridge("deepseek-harness", { ...options, settings: { executable: config.executable } });
  ctx.on("tools/result", safeCallback(async (exec, result) => {
    if (!["win32", "linux", "darwin"].includes(platform) || env.OBIT_DEEPSEEK_LOCAL !== "1" ||
        !["write", "edit"].includes(exec.name) || result?.isError !== false || !exec.agent) return;
    const path = result.value?.path;
    const cwd = ctx.workingDirectory.get(exec.agent.session);
    // The FS seam promises same-file mapping; SSH returns undefined even when
    // the remote path looks like a Windows drive. No WSL/URI path conversion.
    const localPath = (value) => typeof value === "string" &&
      (platform === "win32" ? /^[A-Za-z]:[\\/]/.test(value) : posixPath.isAbsolute(value)) &&
      ctx.fs.processPathFromHostPath(value) === value &&
      exec.agent.ctx?.fs?.processPathFromHostPath(value) === value;
    if (!localPath(path) || !localPath(cwd)) return;
    await bridge.call({ event: "PostToolUse", host_event: "tools/result", tool: exec.name,
      session: exec.agent.session.header.id, cwd, local: true, success: true,
      paths: [path], context_delivery: false });
  }));
}

export const name = "obituary-write-observer";
export const inject = ["fs", "workingDirectory"];
export function apply(ctx, config) { observeDeepSeek(ctx, config); }
