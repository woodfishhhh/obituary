import { nativePath } from "../bridge/native-fixture.mjs";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import vm from "node:vm";

const requests = [];
let reply = { version: 1, deny: true, reason: "obit remove-item -LiteralPath scratch.log" };
let failure = null, raw;
const env = { OBIT_CLINE_LOCAL_SHELL: "powershell" };
const sandbox = vm.createContext({ Buffer, process: { env }, Date });
const cp = new vm.SyntheticModule(["execFile"], function () {
  this.setExport("execFile", (exe, args, options, callback) => {
    assert.equal(exe, nativePath("C:\\Program Files\\obit.exe"));
    assert.deepEqual(Array.from(args), ["hook", "cline"]);
    assert.equal(options.shell, false); assert.equal(options.windowsHide, true);
    assert.ok(options.timeout <= 1500); assert.equal(options.maxBuffer, 16384);
    return { stdin: { on() {}, end(payload) {
      requests.push(JSON.parse(payload));
      callback(failure, raw ?? JSON.stringify(reply));
    } } };
  });
}, { context: sandbox });
const paths = new vm.SyntheticModule(["default"], function () { this.setExport("default", path); }, { context: sandbox });
const module = new vm.SourceTextModule(readFileSync(process.argv[2], "utf8"), { context: sandbox });
await module.link((name) => name === "node:child_process" ? cp : paths);
await module.evaluate();
const plugin = module.namespace.default;
assert.equal(plugin.name, "obituary");
let dispose;
plugin.setup({}, { cwd: nativePath("C:\\repo"), session: { sessionId: "stable" }, onDispose(fn) { dispose = fn; } });
const input = { commands: ["Remove-Item -LiteralPath scratch.log", { command: "rm", args: ["x"] }] };
let result = await plugin.hooks.beforeTool({ tool: { name: "run_commands" }, input });
assert.equal(result.skip, true); assert.equal(result.reason, reply.reason);
assert.equal(input.commands[0], "Remove-Item -LiteralPath scratch.log");
assert.equal(requests[0].session, "stable"); assert.equal(requests[0].shell, "powershell");
assert.equal(requests[0].local, true);
for (const invalid of ["broken", "null", '{"version":2,"deny":true}']) {
  raw = invalid; assert.equal(await plugin.hooks.beforeTool({ tool: { name: "run_commands" }, input }), undefined);
}
raw = undefined; failure = new Error("timeout");
assert.equal(await plugin.hooks.beforeTool({ tool: { name: "run_commands" }, input }), undefined);
failure = null; reply = { version: 1, context: "proposal" };
assert.equal((await plugin.hooks.beforeRun()).appendContext, "proposal");
const count = requests.length;
assert.equal(await plugin.hooks.beforeRun(), undefined); assert.equal(requests.length, count);
for (const result of [{ success: false }, { success: true, error: "failed" }, {}]) {
  await plugin.hooks.afterTool({ tool: { name: "editor" }, input: { path: "scratch.log" }, result });
}
assert.equal(requests.length, count);
result = await plugin.hooks.afterTool({ tool: { name: "editor" }, input: { path: "scratch.log" }, result: { success: true } });
assert.equal(result.appendContext, "proposal"); assert.deepEqual(requests.at(-1).paths, ["scratch.log"]);
await plugin.hooks.afterTool({ tool: { name: "apply_patch" }, input: { input: "*** Add File: good.txt\n+*** Add File: fake.txt\n*** Delete File: old.txt" }, result: { success: true } });
assert.deepEqual(requests.at(-1).paths, ["good.txt"]);
assert.equal(await plugin.hooks.afterRun(), undefined); assert.equal(requests.at(-1).event, "Stop");
assert.equal(await dispose(), undefined); assert.equal(requests.at(-1).event, "SessionEnd");
delete env.OBIT_CLINE_LOCAL_SHELL;
await plugin.hooks.beforeTool({ tool: { name: "run_commands" }, input });
assert.equal(requests.at(-1).local, false);
assert.equal(await plugin.hooks.beforeTool(null), undefined);
assert.equal(await plugin.hooks.afterTool(null), undefined);
console.log("Cline plugin: argv, veto, context, once-per-session, success paths, silent finish, fail-open passed");
