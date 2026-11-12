import { nativePath } from "../bridge/native-fixture.mjs";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import vm from "node:vm";

const requests = [];
let reply = { version: 1, deny: true, reason: "obit rm scratch.log" }, raw, failure;
const sandbox = vm.createContext({ Buffer, process: { env: { OBIT_KILOCODE_LOCAL_SHELL: "bash" } } });
const cp = new vm.SyntheticModule(["execFile"], function () {
  this.setExport("execFile", (exe, args, options, callback) => {
    assert.equal(exe, nativePath("C:\\Program Files\\obit.exe"));
    assert.deepEqual(Array.from(args), ["hook", "kilocode"]);
    assert.equal(options.shell, false); assert.equal(options.windowsHide, true);
    assert.ok(options.timeout < 3000); assert.equal(options.maxBuffer, 16384);
    return { stdin: { on() {}, end(payload) {
      requests.push(JSON.parse(payload)); callback(failure, raw ?? JSON.stringify(reply));
    } } };
  });
}, { context: sandbox });
const paths = new vm.SyntheticModule(["default"], function () { this.setExport("default", path); }, { context: sandbox });
const module = new vm.SourceTextModule(readFileSync(process.argv[2], "utf8"), { context: sandbox });
await module.link((name) => name === "node:child_process" ? cp : paths); await module.evaluate();
const hooks = await module.namespace.default.server({ directory: nativePath("C:\\repo") });
const input = { tool: "bash", sessionID: "stable", callID: "call" }, output = { args: { command: "rm scratch.log" } };
let executed = false;
await assert.rejects(async () => { await hooks["tool.execute.before"](input, output); executed = true; }, /obit rm scratch.log/);
assert.equal(executed, false); assert.equal(output.args.command, "rm scratch.log");
assert.equal(requests[0].local, true); assert.equal(requests[0].shell, "bash");
for (const invalid of ["broken", "null", '{"version":2,"deny":true}']) {
  raw = invalid; assert.equal(await hooks["tool.execute.before"](input, output), undefined);
}
raw = undefined; failure = new Error("timeout");
assert.equal(await hooks["tool.execute.before"](input, output), undefined);
failure = null; reply = { version: 1, context: "ignored" };
await hooks["tool.execute.after"]({ tool: "write", sessionID: "stable", args: { filePath: "scratch.log" } }, { metadata: {} });
assert.deepEqual(requests.at(-1).paths, ["scratch.log"]); assert.equal(requests.at(-1).context_delivery, false);
const count = requests.length;
await hooks["tool.execute.after"]({ tool: "write", sessionID: "stable", args: { filePath: "bad.txt" } }, { metadata: { error: "failure" } });
await hooks["tool.execute.after"]({ tool: "bash", sessionID: "stable", args: { command: "echo fake.txt" } }, { output: "wrote fake.txt", metadata: {} });
assert.equal(requests.length, count);
await hooks["tool.execute.after"]({ tool: "apply_patch", sessionID: "stable", args: { patchText: "*** Add File: good.txt\n+*** Add File: fake.txt\n*** Delete File: old.txt" } }, { metadata: {} });
assert.deepEqual(requests.at(-1).paths, ["good.txt"]);
for (const [type, event] of [["session.created", "SessionStart"], ["session.idle", "Stop"], ["session.deleted", "SessionEnd"]]) {
  assert.equal(await hooks.event({ event: { type, properties: { sessionID: "stable", info: { id: "stable" } } } }), undefined);
  assert.equal(requests.at(-1).event, event);
}
assert.equal(await hooks["tool.execute.before"](null, null), undefined);
assert.equal(await hooks["tool.execute.after"](null, null), undefined);
assert.equal(await hooks.event(null), undefined);
const system = { system: ["existing prompt"] };
await hooks["experimental.chat.system.transform"]({ sessionID: "stable" }, system);
assert.deepEqual(system.system, ["existing prompt", "ignored"]);
assert.equal(requests.at(-1).context_delivery, true);
const delivered = requests.length;
await hooks["experimental.chat.system.transform"]({}, { system: [] });
await hooks["experimental.chat.system.transform"]({ sessionID: "stable" }, {});
sandbox.process.env = {};
const remoteHooks = await module.namespace.default.server({ directory: nativePath("C:\\repo") });
await remoteHooks["experimental.chat.system.transform"]({ sessionID: "stable" }, { system: [] });
assert.equal(requests.length, delivered);
console.log("Kilo plugin: argv, intentional veto reaches host, failed writes, silent lifecycle, fail-open passed");
