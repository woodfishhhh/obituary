// Host callback fixture; real installed source, transport to Go over stdin/stdout.
import { readFileSync } from "node:fs";
import { createInterface } from "node:readline";
import { pathToFileURL } from "node:url";
import path from "node:path";
import vm from "node:vm";
const [modulePath, configPath] = process.argv.slice(2);
const input = JSON.parse(readFileSync(configPath, "utf8"));
const lines = createInterface({ input: process.stdin })[Symbol.asyncIterator]();
const invoke = (_exe, _args, _options, callback) => ({ stdin: { on() {}, async end(payload) {
  console.log(JSON.stringify({ request: JSON.parse(payload) }));
  callback(null, (await lines.next()).value);
} } });
const env = { [`OBIT_${input.agent.toUpperCase()}_LOCAL`]: "1", OBIT_KILOCODE_LOCAL_SHELL: "bash" };
const options = { env, settings: { executable: "C:\\test\\obit.exe" }, execFile: invoke };
const handlers = {};
const api = { on(name, handler) { handlers[name] = handler; }, config: { plugins: { entries: { obituary: {
  hooks: { allowConversationAccess: input.granted !== false, allowPromptModification: input.granted !== false },
} } } } };
let result;
if (input.agent === "kilocode") {
  const context = vm.createContext({ Buffer, process: { env } });
  const cp = new vm.SyntheticModule(["execFile"], function () { this.setExport("execFile", invoke); }, { context });
  const paths = new vm.SyntheticModule(["default"], function () { this.setExport("default", path); }, { context });
  const module = new vm.SourceTextModule(readFileSync(modulePath, "utf8"), { context });
  await module.link(name => name === "node:child_process" ? cp : paths);
  await module.evaluate();
  const hooks = await module.namespace.default.server({ directory: input.cwd });
  await hooks["tool.execute.after"]({ tool: "write", sessionID: "s", args: { filePath: "debug.log" } }, { metadata: {} });
  const output = { system: ["original system"] };
  await hooks["experimental.chat.system.transform"]?.({ sessionID: "s" }, output);
  result = output;
} else {
  const module = await import(pathToFileURL(modulePath).href);
  if (input.agent === "openclaw") {
    module.registerOpenClaw(api, options);
    const ctx = { sessionId: "s", workspaceDir: input.cwd };
    await handlers.after_tool_call({ toolName: "write", params: { path: "debug.log" }, result: {} }, ctx);
    result = await handlers.before_prompt_build?.({}, ctx);
  } else {
    if (input.agent === "omp") api.pi = {};
    module.registerPi(api, options);
    result = await handlers.tool_result({ toolName: input.tool ?? "write", input: { path: input.path }, isError: false },
      { cwd: input.cwd, sessionManager: { getSessionId: () => "s" } });
  }
}
console.log(JSON.stringify({ result: result ?? null }));
process.exit(0);
