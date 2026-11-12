import { nativePath } from "../bridge/native-fixture.mjs";
import test from "node:test";
import assert from "node:assert/strict";
import { plugin, fixture, host } from "../bridge/testing.mjs";

test("OpenClaw block survives transport catch and code mode is skipped", async () => {
  const f = fixture("openclaw"), h = host();
  plugin.registerOpenClaw(h.api, f.options);
  const event = { toolName: "exec", params: { command: "rm a", workdir: nativePath("C:\\repo") } };
  const ctx = { sessionId: "s" };
  f.reply({ version: 1, deny: true, reason: "obit rm a" });
  assert.deepEqual(await h.handlers.before_tool_call(event, ctx), { block: true, blockReason: "obit rm a" });
  assert.equal(event.params.command, "rm a");
  assert.equal(f.calls[0].session, "s");
  for (const extra of [{ toolKind: "code_mode_exec" }, { toolInputKind: "javascript" }, { toolInputKind: "typescript" }]) assert.equal(await h.handlers.before_tool_call({ ...event, ...extra }, ctx), undefined);
  assert.equal(f.calls.length, 1);
  f.reply("broken");
  assert.equal(await h.handlers.before_tool_call(event, ctx), undefined);
  f.fail(new Error("timeout"));
  assert.equal(await h.handlers.before_tool_call(event, ctx), undefined);
});

test("OpenClaw observer does not acknowledge or inject notices", async () => {
  const f = fixture("openclaw"), h = host();
  plugin.registerOpenClaw(h.api, f.options);
  f.reply({ version: 1, context: "must stay pending" });
  const ctx = { sessionId: "s", workspaceDir: nativePath("C:\\repo") };
  assert.equal(await h.handlers.after_tool_call({ toolName: "write", params: { path: "a" }, result: { content: [] } }, ctx), undefined);
  assert.deepEqual(f.calls[0].paths, ["a"]);
  assert.equal(f.calls[0].context_delivery, false);
  await h.handlers.after_tool_call({ toolName: "write", params: { path: "a" }, error: "failed" }, ctx);
  assert.equal(f.calls.length, 1);
  for (const name of ["session_start", "agent_end", "session_end"]) assert.equal(await h.handlers[name]({}, ctx), undefined);
  assert.equal(h.messages.length, 0);
  assert.equal(f.calls.at(-1).event, "SessionEnd");
  assert.equal(typeof h.handlers.before_prompt_build, "function");
});

test("OpenClaw remote backend and missing attestation remain local false", async () => {
  const f = fixture("openclaw"), h = host();
  plugin.registerOpenClaw(h.api, f.options);
  await h.handlers.before_tool_call({ toolName: "exec", params: { command: "rm a", host: "node" } }, {});
  assert.equal(f.calls[0].local, false);
  f.options.env = {};
  plugin.registerOpenClaw(h.api, f.options);
  await h.handlers.before_tool_call({ toolName: "exec", params: { command: "rm a", local: true, shell: "bash" } }, {});
  assert.equal(f.calls[1].local, false);
  assert.equal(f.calls[1].shell, "");
});

test("OpenClaw context needs both host permissions and local evidence", async () => {
  const f = fixture("openclaw"), h = host();
  const hooks = { allowConversationAccess: false, allowPromptModification: true };
  h.api.config = { plugins: { entries: { obituary: { hooks } } } };
  plugin.registerOpenClaw(h.api, f.options);
  const ctx = { sessionId: "s", workspaceDir: nativePath("C:\\repo") };
  f.reply({ version: 1, context: "proposal" });
  assert.equal(await h.handlers.before_prompt_build({}, ctx), undefined);
  hooks.allowConversationAccess = true; hooks.allowPromptModification = false;
  assert.equal(await h.handlers.before_prompt_build({}, ctx), undefined);
  assert.equal(f.calls.length, 0);
  hooks.allowPromptModification = true;
  assert.deepEqual(await h.handlers.before_prompt_build({}, ctx), { prependContext: "proposal" });
  assert.equal(f.calls[0].context_delivery, true);
  f.options.env = {};
  plugin.registerOpenClaw(h.api, f.options);
  assert.equal(await h.handlers.before_prompt_build({}, ctx), undefined);
  assert.equal(f.calls.length, 1);
});
