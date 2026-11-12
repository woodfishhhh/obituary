import { nativePath } from "./native-fixture.mjs";
import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { pathToFileURL } from "node:url";

export const plugin = await import(pathToFileURL(process.env.OBIT_PLUGIN_MODULE).href);
export function fixture(agent) {
  const calls = [];
  let response = { version: 1 };
  let error = null;
  const options = {
    settings: { executable: nativePath("C:\\Program Files\\Obit\\obit.exe"), guardRM: true },
    env: { [`OBIT_${agent.toUpperCase()}_LOCAL`]: "1", [`OBIT_${agent.toUpperCase()}_SHELL`]: "bash", OBIT_OPENCLAW_CWD: nativePath("C:\\repo") },
    execFile(exe, argv, opts, callback) {
      assert.equal(exe, options.settings.executable);
      assert.deepEqual(argv, ["hook", agent]);
      assert.equal(opts.shell, false);
      assert.equal(opts.windowsHide, true);
      assert.ok(opts.timeout < 1500);
      assert.ok(opts.maxBuffer <= 65536);
      const stdin = new EventEmitter();
      stdin.end = (value) => {
        calls.push(JSON.parse(value));
        callback(error, typeof response === "string" ? response : JSON.stringify(response));
      };
      return { stdin };
    },
  };
  return { options, calls, reply(value) { response = value; }, fail(value) { error = value; } };
}

export function host(omp = false) {
  const handlers = {}, messages = [];
  const api = { on(name, fn) { handlers[name] = fn; }, sendMessage(...args) { messages.push(args); } };
  if (omp) api.pi = { settings: {} };
  return { api, handlers, messages, ctx: { cwd: nativePath("C:\\repo"), sessionManager: { getSessionId: () => "session-1" } } };
}
