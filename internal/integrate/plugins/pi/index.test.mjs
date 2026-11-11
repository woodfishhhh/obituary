import test from "node:test";
import assert from "node:assert/strict";
import path from "node:path";
import { plugin, fixture, host } from "../bridge/testing.mjs";

for (const agent of ["pi", "omp"]) {
  test(`${agent} skips host-dependent path spellings`, async () => {
    const f = fixture(agent), h = host(agent === "omp");
    plugin.registerPi(h.api, f.options);
    for (const target of ["~/x.log", "file:///C:/x.log", "/mnt/c/x.log", "C:relative.log", "scratch.log:3"]) {
      await h.handlers.tool_result({ toolName: "write", input: { path: target }, isError: false }, h.ctx);
    }
    if (agent === "omp") {
      for (const target of ["@C:/repo/x.log", "local://plan", "[x.log#TAG]"]) {
        await h.handlers.tool_result({ toolName: "edit", input: { path: target }, isError: false }, h.ctx);
      }
    }
    assert.equal(f.calls.length, 0);
  });
  test(`${agent} denies without changing input, and transport errors pass`, async () => {
    const f = fixture(agent), h = host(agent === "omp");
    plugin.registerPi(h.api, f.options);
    const event = { toolName: "bash", input: { command: 'rm "a b"' } };
    f.reply({ version: 1, deny: true, reason: 'obit rm "a b"' });
    assert.deepEqual(await h.handlers.tool_call(event, h.ctx), { block: true, reason: 'obit rm "a b"' });
    assert.equal(event.input.command, 'rm "a b"');
    assert.equal(f.calls[0].agent, agent);
    assert.equal(f.calls[0].local, true);
    assert.equal(f.calls[0].session, "session-1");
    for (const reply of ["broken", { version: 9, deny: true }, { version: 1, deny: "true", reason: "bad" }]) {
      f.reply(reply);
      assert.equal(await h.handlers.tool_call(event, h.ctx), undefined);
    }
    f.fail(new Error("timeout"));
    assert.equal(await h.handlers.tool_call(event, h.ctx), undefined);
  });
  test(`${agent} guards opt in and locality is explicit`, async () => {
    const f = fixture(agent), h = host(agent === "omp");
    f.options.settings.guardRM = false;
    plugin.registerPi(h.api, f.options);
    await h.handlers.tool_call({ toolName: "bash", input: { command: "rm a" } }, h.ctx);
    assert.equal(f.calls.length, 0);
    f.options.settings.guardRM = true;
    f.options.env = {};
    plugin.registerPi(h.api, f.options);
    await h.handlers.tool_call({ toolName: "bash", input: { command: "rm a", local: true, shell: "bash" } }, h.ctx);
    assert.equal(f.calls[0].local, false);
    assert.equal(f.calls[0].shell, "");
  });
  test(`${agent} successful input paths and passive lifecycle`, async () => {
    const f = fixture(agent), h = host(agent === "omp");
    plugin.registerPi(h.api, f.options);
    f.reply({ version: 1, context: "pending proposal" });
    const result = await h.handlers.tool_result({ toolName: "write", input: { path: "scratch.txt" }, isError: false, content: [{ type: "text", text: "ok" }] }, h.ctx);
    assert.deepEqual(f.calls[0].paths, [path.resolve(h.ctx.cwd, "scratch.txt")]);
    if (agent === "omp") assert.deepEqual(result, { additionalContext: "pending proposal" });
    else assert.equal(result.content[1].text, "pending proposal");
    const count = f.calls.length;
    await h.handlers.tool_result({ toolName: "write", input: { path: "scratch.txt" }, isError: true }, h.ctx);
    await h.handlers.tool_result({ toolName: "bash", input: { command: "echo fake" }, isError: false, content: [{ text: "scratch.txt" }] }, h.ctx);
    await h.handlers.tool_result({ toolName: "write", input: { path: "scratch.txt", host: "ssh" }, isError: false }, h.ctx);
    assert.equal(f.calls.length, count);
    await h.handlers.session_start({}, h.ctx);
    assert.equal(h.messages[0][1].triggerTurn, false);
    const messages = h.messages.length;
    for (const event of ["agent_end", agent === "omp" ? "session_stop" : "agent_settled", "session_shutdown"]) assert.equal(await h.handlers[event]({}, h.ctx), undefined);
    assert.equal(h.messages.length, messages);
    assert.equal(f.calls.at(-1).event, "SessionEnd");
    assert.equal(f.calls.at(-1).context_delivery, false);
  });
}
