import { nativePath } from "../bridge/native-fixture.mjs";
import test from 'node:test';
import assert from 'node:assert/strict';
import { pathToFileURL } from 'node:url';
const { observeDeepSeek } = await import(pathToFileURL(process.env.OBIT_PLUGIN_MODULE));

function setup(options = {}) {
  const calls = [];
  let handler;
  const ctx = {
    on(event, callback) { assert.equal(event, 'tools/result'); handler = callback; },
    fs: { processPathFromHostPath: options.mapping ?? (value => value) },
    workingDirectory: { get: () => options.cwd ?? 'C:\\repo' },
  };
  const execFile = (executable, argv, settings, callback) => {
    assert.equal(executable, nativePath('C:\\tools\\obit.exe'));
    assert.deepEqual(argv, ['hook', 'deepseek-harness']);
    assert.equal(settings.shell, false);
    assert.equal(settings.windowsHide, true);
    assert.equal(settings.timeout, 1200);
    return { stdin: { on() {}, end(raw) {
      calls.push(JSON.parse(raw));
      callback(options.failure ? new Error('timeout') : null, options.reply ?? '{"version":1}');
    } } };
  };
  observeDeepSeek(ctx, { executable: nativePath('C:\\tools\\obit.exe') }, {
    env: { OBIT_DEEPSEEK_LOCAL: options.local ?? '1' }, platform: options.platform ?? 'win32', execFile,
  });
  const run = (exec = {}, result = {}) => handler({
    name: 'write', arguments: { file_path: 'C:\\repo\\untrusted.log' },
    agent: { ctx: { fs: { processPathFromHostPath: options.agentMapping ?? ctx.fs.processPathFromHostPath } }, session: { header: { id: 'session-1' } } }, ...exec,
  }, { isError: false, value: { path: 'C:\\repo\\written.log' }, ...result });
  return { run, calls };
}

test('native POSIX canonical writes require the same-file mapping', { skip: process.platform === 'win32' }, async () => {
  const s = setup({ platform: process.platform, cwd: '/repo' });
  await s.run({}, { value: { path: '/repo/written.log' } });
  assert.equal(s.calls.length, 1);
  assert.deepEqual(s.calls[0].paths, ['/repo/written.log']);
  const remote = setup({ platform: process.platform, cwd: '/repo', mapping: () => undefined });
  await remote.run({}, { value: { path: '/repo/written.log' } });
  assert.equal(remote.calls.length, 0);
});

test('successful canonical write/edit paths defer context through fixed argv', async () => {
  for (const name of ['write', 'edit']) {
    const s = setup(); await s.run({ name });
    assert.equal(s.calls.length, 1);
    const payload = s.calls[0];
    assert.equal(payload.version, 1);
    assert.equal(payload.agent, 'deepseek-harness');
    assert.equal(payload.host_event, 'tools/result');
    assert.equal(payload.local, true);
    assert.equal(payload.success, true);
    assert.equal(payload.context_delivery, false);
    assert.deepEqual(payload.paths, ['C:\\repo\\written.log']);
    assert.equal(payload.session, 'session-1');
  }
});

test('errors, missing canonical result, unsupported tools and absent agents stay silent', async () => {
  for (const [exec, result] of [
    [{}, { isError: true }], [{}, { isError: undefined }], [{}, { value: undefined }],
    [{ name: 'bash' }, {}], [{ agent: undefined }, {}],
    [{}, { value: { path: '/mnt/c/repo/file.log' } }],
    [{}, { value: { path: 'ssh://host/C:/repo/file.log' } }],
  ]) {
    const s = setup(); await s.run(exec, result); assert.equal(s.calls.length, 0);
  }
});

test('SSH and remote Windows paths fail same-file mapping even under local opt-in', async () => {
  for (const options of [
    { mapping: () => undefined }, { mapping: () => 'D:\\remote\\file.log' },
    { agentMapping: () => undefined },
    { mapping: () => { throw new Error('unavailable'); } },
    { cwd: '/mnt/c/repo' }, { platform: 'linux' }, { local: '' }, { local: 'true' },
  ]) {
    const s = setup(options); await s.run(); assert.equal(s.calls.length, 0);
  }
});

test('transport errors and malformed replies leave the host result untouched', async () => {
  for (const options of [{ failure: true }, { reply: 'broken' }, { reply: '{"version":2,"deny":true}' }]) {
    const s = setup(options);
    const result = { isError: false, value: { path: 'C:\\repo\\written.log' }, content: [{ type: 'text', text: 'original' }] };
    const before = JSON.stringify(result);
    await s.run({}, result);
    assert.equal(JSON.stringify(result), before);
  }
});
