import { nativePath } from "../bridge/native-fixture.mjs";
import test from "node:test";
import assert from "node:assert/strict";
import { plugin, fixture } from "../bridge/testing.mjs";

class ToolError extends Error {
  _tag = "Tool.Error";
  constructor({ message }) { super(message); }
}

test("OpenCode V2 rejects via Tool.Error outside transport catch", async () => {
  const f = fixture("opencode");
  f.options.ToolError = ToolError;
  const tool = {}, session = {};
  await plugin.createOpenCode(f.options).setup({ location: { directory: nativePath("C:\\repo") }, tool: { hook(n, cb) { tool[n] = cb; } }, session: { hook(n, cb) { session[n] = cb; } } });
  const event = { tool: "bash", sessionID: "s", input: { command: "rm a" } };
  f.reply({ version: 1, deny: true, reason: "obit rm a" });
  await assert.rejects(tool["execute.before"](event), e => e._tag === "Tool.Error" && e.message === "obit rm a");
  assert.equal(event.input.command, "rm a");
  assert.equal(f.calls[0].cwd, nativePath("C:\\repo"));
  f.reply("bad json");
  await tool["execute.before"](event);
  f.fail(new Error("timeout"));
  await tool["execute.before"](event);
  f.fail(null);
  f.reply({ version: 1, context: "context" });
  const success = { tool: "write", sessionID: "s", input: { filePath: "a" }, status: "completed", result: { content: [] } };
  await tool["execute.after"](success);
  assert.equal(success.result.content[0].text, "context");
  const count = f.calls.length;
  await tool["execute.after"]({ ...success, status: "error" });
  assert.equal(f.calls.length, count);
  await session.prompt({ sessionID: "s" });
  await session.prompt({ sessionID: "s" });
  assert.equal(f.calls.filter(c => c.event === "SessionStart").length, 1);
  assert.equal(await session["experimental.idle"]({ sessionID: "s" }), undefined);
  assert.equal(f.calls.at(-1).event, "Stop");
});

test("OpenCode V1 preserves arguments and model-visible output", async () => {
  const f = fixture("opencode"), p = plugin.createOpenCode(f.options);
  assert.equal(typeof p, "object");
  assert.equal(p.id, "obituary");
  const hooks = await p.server({ directory: nativePath("C:\\repo") });
  const event = { tool: "bash", sessionID: "s" }, output = { args: { command: "rm a" } };
  f.reply({ version: 1, deny: true, reason: "obit rm a" });
  await assert.rejects(hooks["tool.execute.before"](event, output), /obit rm a/);
  assert.equal(output.args.command, "rm a");
  f.reply({ version: 1, context: "notice" });
  const result = { output: "ok", metadata: {} };
  await hooks["tool.execute.after"]({ tool: "edit", sessionID: "s", args: { filePath: "a" } }, result);
  assert.equal(result.output, "ok\nnotice");
  for (const type of ["session.created", "session.idle", "session.deleted"]) assert.equal(await hooks.event({ event: { type, properties: { sessionID: "s" } } }), undefined);
  assert.equal(f.calls.at(-1).event, "SessionEnd");
  f.fail(new Error("spawn"));
  assert.equal(await hooks["tool.execute.before"](event, output), undefined);
});

test("OpenCode ignores tool-claimed locality and guard defaults off", async () => {
  const f = fixture("opencode");
  f.options.env = {};
  const hooks = await plugin.createOpenCode(f.options).server({ directory: nativePath("C:\\repo") });
  await hooks["tool.execute.before"]({ tool: "bash" }, { args: { command: "rm a", local: true, shell: "bash" } });
  assert.equal(f.calls[0].local, false);
  assert.equal(f.calls[0].shell, "");
  f.options.settings.guardRM = false;
  const disabled = await plugin.createOpenCode(f.options).server();
  await disabled["tool.execute.before"]({ tool: "bash" }, { args: { command: "rm a" } });
  assert.equal(f.calls.length, 1);
});
