import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { execFile, spawn } from 'node:child_process';
import { once } from 'node:events';
import { createServer } from 'node:http';
import { copyFile, link, mkdir, readFile, readdir, stat, symlink, writeFile } from 'node:fs/promises';
import { join, resolve } from 'node:path';
import { promisify } from 'node:util';
import test from 'node:test';

const execute = promisify(execFile);
const sha256 = data => createHash('sha256').update(data).digest('hex');

// Windows integration test; first build the snapshot and set
// OBIT_INSTALL_TEST_ROOT to a disposable directory created by obit mktemp.
test('install verified snapshot releases without changing the user PATH', {
  skip: process.platform !== 'win32',
}, async t => {
  assert.ok(process.env.OBIT_INSTALL_TEST_ROOT, 'Set OBIT_INSTALL_TEST_ROOT to an obit mktemp directory');
  const root = join(process.env.OBIT_INSTALL_TEST_ROOT, `install-tests-${Date.now()}`);
  await mkdir(root, { recursive: true });
  const env = { ...process.env };
  // A pwsh parent can expose its Core modules to Windows PowerShell 5.1.
  // Each engine must discover its own built-in modules, without profiles.
  for (const key of Object.keys(env)) {
    if (key.toUpperCase() === 'PSMODULEPATH') delete env[key];
  }
  const readUserEnvironment = async () => (await execute('reg.exe', ['query', 'HKCU\\Environment'], {
    encoding: 'buffer', windowsHide: true,
  })).stdout;
  const userEnvironmentBefore = await readUserEnvironment();
  for (const name of ['OBIT_HOME', 'OBIT_CONFIG_DIR', 'OBIT_USER_HOME', 'CLAUDE_CONFIG_DIR', 'CODEX_HOME']) {
    env[name] = join(root, name);
    await mkdir(env[name]);
  }
  const downloads = join(root, 'downloads');
  await mkdir(downloads);
  env.TEMP = env.TMP = downloads;
  const dist = resolve(process.env.OBIT_INSTALL_TEST_DIST ?? 'bin/release');
  const manifest = JSON.parse(await readFile(join(dist, 'scoop/obituary.json'), 'utf8'));
  const version = manifest.version;
  const archiveName = `obituary_${version}_windows_amd64.zip`;
  const zip = await readFile(join(dist, archiveName));
  const armArchiveName = `obituary_${version}_windows_arm64.zip`;
  const armZip = await readFile(join(dist, armArchiveName));
  const checksums = await readFile(join(dist, 'checksums.txt'));
  const checksumEntry = `${sha256(zip)}  ${archiveName}\n`;
  const downloadsBefore = new Set(await readdir(downloads));
  let requests = 0;
  const seen = [];
  const server = createServer((req, res) => {
    requests++;
    seen.push(req.url);
    const [, scenario, file] = req.url.split('/');
    if (scenario === 'not-found') {
      res.writeHead(404).end();
    } else if (file === 'checksums.txt') {
      const variants = {
        missing: checksumEntry.replace(archiveName, `${archiveName}.backup`),
        duplicate: checksumEntry + checksumEntry,
        malformed: `not-a-sha256  ${archiveName}\n`,
        incomplete: `${sha256(armZip)}  ${archiveName}\n`,
      };
      res.end(variants[scenario] ?? checksums);
    } else if (file === archiveName) {
      const variants = { corrupt: Buffer.concat([zip, Buffer.from('corrupt')]), incomplete: armZip };
      res.end(variants[scenario] ?? zip);
    } else if (file === armArchiveName) {
      res.end(armZip);
    } else {
      res.writeHead(404).end();
    }
  });
  await new Promise(resolveListen => server.listen(0, '127.0.0.1', resolveListen));
  t.after(() => new Promise(resolveClose => server.close(resolveClose)));
  const base = `http://127.0.0.1:${server.address().port}`;
  const ps7 = join(process.env.ProgramFiles, 'PowerShell/7/pwsh.exe');
  const ps5 = join(process.env.SystemRoot, 'System32/WindowsPowerShell/v1.0/powershell.exe');
  const engines = [['ps7', ps7], ['ps5', ps5]].filter(([name]) =>
    !process.env.OBIT_INSTALL_TEST_ENGINES || process.env.OBIT_INSTALL_TEST_ENGINES.split(',').includes(name));
  const install = join(root, 'installed');
  async function shortcut(destination, shell = ps7, target) {
    const result = await execute(shell, ['-NoProfile', '-NonInteractive', '-File',
      resolve('scripts/testdata/shortcut.ps1'), '-Path', join(`${destination}-menu`, '讣告 Obituary.lnk'),
      ...(target ? ['-ReplaceTarget', target] : [])], { env, windowsHide: true });
    return JSON.parse(result.stdout);
  }
  let runs = 0;
  // Replace only the OS PATH boundary in a test copy. No test branch can call
  // Environment.SetEnvironmentVariable(..., User) against the real registry.
  const sourceScript = join(root, 'install-under-test.ps1');
  await copyFile(resolve(process.env.OBIT_INSTALL_TEST_SCRIPT ?? 'packaging/install.ps1'), sourceScript);
  const pathScript = join(root, 'install-path-fixture.ps1');
  const source = await readFile(sourceScript, 'utf8');
  const isolatedPath = source
    .replaceAll("[Environment]::GetEnvironmentVariable('Path', 'User')", "[IO.File]::ReadAllText($env:OBIT_INSTALL_PATH_FIXTURE)")
    .replaceAll(/\[Environment\]::SetEnvironmentVariable\('Path', (\$\w+), 'User'\)/g, "[IO.File]::WriteAllText($env:OBIT_INSTALL_PATH_FIXTURE, $1)");
  assert.ok(!isolatedPath.includes("[Environment]::SetEnvironmentVariable('Path'"));
  await writeFile(pathScript, isolatedPath);
  async function run(scenario, extra = [], shell = ps7, destination = install, architecture = 'amd64', script = sourceScript) {
    const pathFixture = extra.includes('-FixturePath');
    // -DefaultSource leaves out -BaseUrl, so the script picks its own source.
    const defaultSource = extra.includes('-DefaultSource');
    extra = extra.filter(arg => arg !== '-FixturePath' && arg !== '-DefaultSource');
    const args = ['-NoProfile', '-NonInteractive', '-ExecutionPolicy', 'Bypass', '-File',
      pathFixture ? pathScript : script,
      ...(extra.includes('-Uninstall') ? [] : defaultSource ? ['-Version', version] : ['-Version', version, '-BaseUrl', `${base}/${scenario}`]),
      '-Architecture', architecture, '-InstallDir', destination,
      '-StartMenuDir', `${destination}-menu`, ...(pathFixture ? [] : ['-NoPath']), ...extra];
    let result;
    try {
      result = { code: 0, ...await execute(shell, args, { env, windowsHide: true }) };
    } catch (error) {
      if (typeof error.code !== 'number') throw error;
      result = error;
    }
    await writeFile(join(root, `${++runs}-${scenario}-${shell === ps7 ? 'ps7' : 'ps5'}.log`),
      `exit=${result.code}\n${result.stdout}\n${result.stderr}`, 'utf8');
    return result;
  }

  await t.test('WhatIf performs no download and creates no install directory', async () => {
    const result = await run('good', ['-WhatIf']);
    assert.equal(result.code, 0, result.stderr);
    assert.match(result.stdout, /What if:/i);
    assert.equal(requests, 0);
    await assert.rejects(stat(install), { code: 'ENOENT' });
    await assert.rejects(stat(`${install}-menu`), { code: 'ENOENT' });
  });
  await t.test('SHA-256 manifests describe the actual snapshot archives', async () => {
    assert.equal(manifest.architecture['64bit'].hash.toLowerCase(), sha256(zip));
    assert.equal(manifest.architecture.arm64.hash.toLowerCase(), sha256(armZip));
    assert.deepEqual([...manifest.architecture['64bit'].bin].sort(), ['obit-gui.exe', 'obit.exe', 'obitd.exe']);
    assert.deepEqual([...manifest.architecture.arm64.bin].sort(), ['obit.exe', 'obitd.exe']);
    assert.ok(manifest.notes.some(note => note.includes('obit uninstall --yes')));
    assert.deepEqual(manifest.architecture['64bit'].shortcuts, [['obit-gui.exe', 'Obituary', '', 'obit-gui.exe']]);
    assert.equal(manifest.architecture.arm64.shortcuts, undefined);
    const winget = await readFile(join(dist, 'winget/manifests/w/woodfishhhh/Obituary', version,
      'woodfishhhh.Obituary.installer.yaml'), 'utf8');
    assert.match(winget, /^ArchiveBinariesDependOnPath: true$/m);
    assert.ok(winget.includes(`InstallerSha256: ${sha256(zip)}`));
    assert.ok(winget.includes(`InstallerSha256: ${sha256(armZip)}`));
  });
  await t.test('install all amd64 binaries together and run the installed CLI', async () => {
    const result = await run('good');
    assert.equal(result.code, 0, result.stderr);
    assert.match(result.stdout, /SHA-256 verified:/);
    assert.deepEqual((await readdir(install)).sort(), ['.obituary-install.json', 'LICENSE', 'README.txt', 'obit-gui.exe', 'obit.exe', 'obitd.exe']);
    const { stdout } = await execute(join(install, 'obit.exe'), ['--version'], { env, windowsHide: true });
    assert.ok(stdout.includes(version), stdout);
    await execute(join(install, 'obit.exe'), ['daemon', 'stop'], { env, windowsHide: true });
    await stat(join(`${install}-menu`, '讣告 Obituary.lnk'));
    const link = await shortcut(install);
    assert.equal(link.target.toLowerCase(), join(install, 'obit-gui.exe').toLowerCase());
    assert.equal(link.workingDirectory.toLowerCase(), install.toLowerCase());
    assert.equal(link.icon.toLowerCase(), `${join(install, 'obit-gui.exe')},0`.toLowerCase());
  });
  const installedHash = sha256(await readFile(join(install, 'obit.exe')));
  await t.test('arm64 installs CLI and daemon without requiring an unavailable GUI', async () => {
    const destination = join(root, 'installed-arm64');
    const result = await run('good', [], ps7, destination, 'arm64');
    assert.equal(result.code, 0, result.stderr);
    assert.deepEqual((await readdir(destination)).sort(), ['.obituary-install.json', 'LICENSE', 'README.txt', 'obit.exe', 'obitd.exe']);
    await assert.rejects(stat(`${destination}-menu`), { code: 'ENOENT' });
  });
  await t.test('repeat installation replaces an older binary and preserves unrelated files', async () => {
    await writeFile(join(install, 'obit.exe'), 'old release');
    await writeFile(join(install, 'keep.txt'), 'user file');
    const result = await run('upgrade', [], ps7, install, 'AMD64');
    assert.equal(result.code, 0, result.stderr);
    assert.equal(sha256(await readFile(join(install, 'obit.exe'))), installedHash);
    assert.equal(await readFile(join(install, 'keep.txt'), 'utf8'), 'user file');
  });
  for (const [scenario, message] of [
    ['corrupt', /SHA-256 mismatch/],
    ['missing', /exactly one SHA-256 entry/],
    ['duplicate', /exactly one SHA-256 entry/],
    ['malformed', /exactly one SHA-256 entry/],
    ['incomplete', /missing obit-gui.exe/],
    ['not-found', /404/],
  ]) {
    await t.test(`reject ${scenario} before touching the existing installation`, async () => {
      const result = await run(scenario);
      assert.notEqual(result.code, 0);
      assert.match(result.stderr, message);
      assert.equal(sha256(await readFile(join(install, 'obit.exe'))), installedHash);
      assert.equal(await readFile(join(install, 'keep.txt'), 'utf8'), 'user file');
    });
  }
  // Without -BaseUrl the release comes from GitHub, and from the mainland
  // China mirror when GitHub gives no answer. Test copies point "GitHub" at a
  // closed port (no answer) or at a 404 (an answer), and the mirror at /mirror/.
  const closed = createServer();
  await new Promise(resolveListen => closed.listen(0, '127.0.0.1', resolveListen));
  const closedUrl = `http://127.0.0.1:${closed.address().port}`;
  await new Promise(resolveClose => closed.close(resolveClose));
  async function sourcesScript(name, github) {
    const file = join(root, `install-${name}.ps1`);
    const text = source
      .replace('https://github.com/woodfishhhh/obituary/releases/download/v$Version/', `${github}/v$Version/`)
      .replace('https://obituary.woodfish.site/download/v$Version/', `${base}/mirror/`);
    assert.ok(!text.includes('https://github.com/') && !text.includes('https://obituary.woodfish.site/'));
    await writeFile(file, text);
    return file;
  }
  const mirrorRequests = from => seen.slice(from).filter(url => url.startsWith('/mirror/'));
  const installedNames = ['.obituary-install.json', 'LICENSE', 'README.txt', 'obit-gui.exe', 'obit.exe', 'obitd.exe'];
  for (const [engine, shell] of engines) {
    await t.test(`${engine} falls back to the mirror when GitHub does not answer`, async () => {
      const destination = join(root, `${engine}-fallback`);
      const from = seen.length;
      const result = await run('fallback', ['-DefaultSource'], shell, destination, 'amd64', await sourcesScript('fallback', closedUrl));
      assert.equal(result.code, 0, result.stderr);
      assert.match(result.stdout, /GitHub did not answer/);
      assert.match(result.stdout, /SHA-256 verified:/);
      assert.deepEqual(mirrorRequests(from), ['/mirror/checksums.txt', `/mirror/${archiveName}`]);
      assert.deepEqual((await readdir(destination)).sort(), installedNames);
    });
  }
  await t.test('an HTTP error from GitHub is reported, not retried on the mirror', async () => {
    const from = seen.length;
    const result = await run('github-404', ['-DefaultSource'], ps7, join(root, 'github-404'), 'amd64', await sourcesScript('github-404', `${base}/not-found`));
    assert.notEqual(result.code, 0);
    assert.match(result.stderr, /404/);
    assert.deepEqual(mirrorRequests(from), []);
    await assert.rejects(stat(join(root, 'github-404')), { code: 'ENOENT' });
  });
  await t.test('-Mirror cn downloads from the mirror without asking GitHub', async () => {
    const destination = join(root, 'mirror-cn');
    const from = seen.length;
    const result = await run('mirror-cn', ['-DefaultSource', '-Mirror', 'cn'], ps7, destination, 'amd64', await sourcesScript('mirror-cn', closedUrl));
    assert.equal(result.code, 0, result.stderr);
    assert.doesNotMatch(result.stdout, /GitHub did not answer/);
    assert.deepEqual(mirrorRequests(from), ['/mirror/checksums.txt', `/mirror/${archiveName}`]);
    assert.deepEqual((await readdir(destination)).sort(), installedNames);
  });
  await t.test('-Mirror and -BaseUrl together are refused before any download', async () => {
    const from = seen.length;
    const result = await run('good', ['-Mirror', 'cn'], ps7, join(root, 'mirror-and-base'));
    assert.notEqual(result.code, 0);
    assert.match(result.stderr, /either -Mirror or -BaseUrl/);
    assert.equal(seen.length, from);
  });
  await t.test('Windows PowerShell 5.1 installs the same verified release', {skip: !engines.some(([name]) => name === 'ps5')}, async () => {
    const result = await run('good', [], ps5, join(root, 'installed-ps5'));
    assert.equal(result.code, 0, result.stderr);
    assert.equal(sha256(await readFile(join(root, 'installed-ps5/obit.exe'))), installedHash);
  });
  const files = ['obit.exe', 'obitd.exe', 'LICENSE', 'README.txt', 'obit-gui.exe'];
  for (const [engine, shell] of engines) {
    for (const kind of ['hardlink', 'junction', 'directory', 'install-junction', 'parent-junction']) {
      await t.test(`${engine} rejects ${kind} and preserves external content`, async () => {
        const area = join(root, `${engine}-${kind}`);
        let destination = join(area, 'installed');
        const outside = join(area, 'outside');
        await mkdir(destination, { recursive: true });
        await mkdir(outside);
        const sentinel = join(outside, 'obitd.exe');
        await writeFile(sentinel, 'USER CONTENT');
        if (kind === 'hardlink') await link(sentinel, join(destination, 'obitd.exe'));
        if (kind === 'junction') await symlink(outside, join(destination, 'obitd.exe'), 'junction');
        if (kind === 'directory') {
          await mkdir(join(destination, 'obitd.exe'));
          await writeFile(join(destination, 'obitd.exe/obitd.exe'), 'USER CONTENT');
        }
        if (kind.endsWith('-junction')) {
          const junction = join(area, 'linked');
          await symlink(outside, junction, 'junction');
          destination = kind === 'install-junction' ? junction : join(junction, 'installed');
        }
        const result = await run('good', [], shell, destination);
        assert.equal(await readFile(sentinel, 'utf8'), 'USER CONTENT');
        if (kind === 'directory') {
          assert.equal(await readFile(join(destination, 'obitd.exe/obitd.exe'), 'utf8'), 'USER CONTENT');
        }
        assert.notEqual(result.code, 0, 'unsafe installation must fail');
        assert.match(result.stderr, kind === 'hardlink' ? /hard link/i : kind === 'directory' ? /directory/i : /reparse point/i);
      });
    }
  }
  const locker = join(root, 'install-lock.exe');
  await execute('go', ['build', '-o', locker, './scripts/testdata/install-lock.go'], { env, windowsHide: true });
  for (const [engine, shell] of engines) {
    await t.test(`${engine} restores all five old files when the second file is locked`, async () => {
      const destination = join(root, `${engine}-locked`);
      await mkdir(destination);
      for (const name of files) await writeFile(join(destination, name), `OLD-${name}`);
      const child = spawn(locker, [join(destination, 'obitd.exe')], { env, windowsHide: true, stdio: ['pipe', 'pipe', 'pipe'] });
      const exited = once(child, 'exit');
      try {
        const [ready] = await Promise.race([
          once(child.stdout, 'data'),
          exited.then(([code]) => { throw new Error(`lock helper exited before readiness: ${code}`); }),
        ]);
        assert.match(ready.toString(), /locked/);
        const result = await run('good', [], shell, destination);
        assert.notEqual(result.code, 0, 'exclusive lock must fail installation');
      } finally {
        child.stdin.end();
        await exited;
      }
      for (const name of files) {
        assert.ok((await readFile(join(destination, name))).equals(Buffer.from(`OLD-${name}`)), `${name} changed after failed upgrade`);
      }
      assert.deepEqual((await readdir(destination)).sort(), [...files].sort());
    });
  }
  await t.test('all successful and failed downloads are removed', async () => {
    const remaining = (await readdir(downloads)).filter(name => !downloadsBefore.has(name));
    assert.deepEqual(remaining, []);
  });

  // The fake CLI records the exact uninstall arguments. Its unit tests exercise
  // the real command with fake registries/tasks/tray; packaging tests must never
  // call the production registry or scheduled-task boundary.
  const uninstaller = join(root, 'fake-uninstall.exe');
  await execute('go', ['build', '-o', uninstaller, './scripts/testdata/install-uninstall.go'], { env, windowsHide: true });
  const callLog = join(root, 'uninstall-calls.json');
  env.OBIT_INSTALL_UNINSTALL_LOG = callLog;
  async function prepareUninstall(destination, shell = ps7, extra = []) {
    const result = await run('good', extra, shell, destination);
    assert.equal(result.code, 0, result.stderr);
    await copyFile(uninstaller, join(destination, 'obit.exe'));
    const receiptPath = join(destination, '.obituary-install.json');
    const receipt = JSON.parse(await readFile(receiptPath, 'utf8'));
    receipt.files.find(file => file.name === 'obit.exe').sha256 = sha256(await readFile(uninstaller));
    await writeFile(receiptPath, JSON.stringify(receipt));
  }
  for (const [engine, shell] of engines) {
    await t.test(`${engine} shortcut validation and deletion use the same object`, async () => {
      const destination = join(root, `${engine}-shortcut-swap`);
      await prepareUninstall(destination, shell);
      const raceScript = join(root, `${engine}-shortcut-race.ps1`);
      env.OBIT_INSTALL_SWAP_LOG = join(root, `${engine}-shortcut-swap.txt`);
      const boundary = '[ObituaryInstall]::Delete($file)';
      assert.equal(source.split(boundary).length, 2, 'shortcut deletion boundary moved');
      await writeFile(raceScript, source.replace(boundary, `
        $replaced = $false
        try {
            [IO.File]::Move($shortcutPath, $shortcutPath + '.saved')
            [IO.File]::WriteAllText($shortcutPath, 'USER REPLACEMENT')
            $replaced = $true
        } catch [IO.IOException] { }
        [IO.File]::WriteAllText($env:OBIT_INSTALL_SWAP_LOG, [string]$replaced)
        ${boundary}`));
      const result = await run('remove', ['-Uninstall'], shell, destination, 'amd64', raceScript);
      assert.equal(result.code, 0, result.stderr);
      assert.equal(await readFile(env.OBIT_INSTALL_SWAP_LOG, 'utf8'), 'False', 'verified shortcut could be replaced');
    });
    await t.test(`${engine} a mid-delete failure retains the executable and permits retry`, async () => {
      const destination = join(root, `${engine}-delete-failure`);
      await prepareUninstall(destination, shell);
      const faultScript = join(root, `${engine}-delete-fault.ps1`);
      const boundary = '[ObituaryInstall]::Delete($file.Stream)';
      assert.equal(source.split(boundary).length, 2, 'payload deletion boundary moved');
      await writeFile(faultScript, source.replace(boundary, `
        if ($file.Name -ceq 'obit-gui.exe') { throw 'injected deletion failure' }
        ${boundary}`));
      let result = await run('remove', ['-Uninstall'], shell, destination, 'amd64', faultScript);
      assert.notEqual(result.code, 0);
      assert.match(result.stderr, /injected deletion failure/);
      await stat(join(destination, 'obit.exe'));
      await stat(join(destination, '.obituary-install.json'));
      result = await run('remove', ['-Uninstall'], shell, destination);
      assert.equal(result.code, 0, result.stderr);
      await assert.rejects(stat(destination), {code: 'ENOENT'});
    });
    for (const state of (engine === 'ps7' ? ['added', 'upgraded', 'other files', 'edited', 'duplicate', 'legacy'] : ['added'])) {
      await t.test(`${engine} PATH ownership lifecycle: ${state}`, async () => {
        const destination = join(root, `${engine}-path-${state.replaceAll(' ', '-')}`);
        env.OBIT_INSTALL_PATH_FIXTURE = join(root, `${engine}-path-${state}.txt`);
        const original = 'C:\\Other;C:\\More;';
        await writeFile(env.OBIT_INSTALL_PATH_FIXTURE, original);
        await prepareUninstall(destination, shell, ['-FixturePath']);
        if (state === 'upgraded') await prepareUninstall(destination, shell, ['-FixturePath']);
        const receiptPath = join(destination, '.obituary-install.json');
        const receipt = JSON.parse(await readFile(receiptPath, 'utf8'));
        assert.equal(receipt.pathAdded, true);
        if (state === 'other files') await writeFile(join(destination, 'another.exe'), 'USER APPLICATION');
        if (state === 'edited') await writeFile(env.OBIT_INSTALL_PATH_FIXTURE, `${original};"${destination}"`);
        if (state === 'duplicate') await writeFile(env.OBIT_INSTALL_PATH_FIXTURE, `${original};${destination};${destination}`);
        if (state === 'legacy') {
          delete receipt.pathAdded; delete receipt.pathEntry;
          await writeFile(receiptPath, JSON.stringify(receipt));
        }
        const before = await readFile(env.OBIT_INSTALL_PATH_FIXTURE, 'utf8');
        const result = await run('remove', ['-Uninstall', '-FixturePath'], shell, destination);
        assert.equal(result.code, 0, result.stderr);
        assert.equal(await readFile(env.OBIT_INSTALL_PATH_FIXTURE, 'utf8'), ['added', 'upgraded'].includes(state) ? original : before);
      });
    }
    await t.test(`${engine} TestReviewInstallerDirectoryPinDoesNotPinChild`, async () => {
      const destination = join(root, `${engine}-uninstall-swaps`);
      await prepareUninstall(destination, shell);
      const paths = ['README.txt', 'obit.exe'].map(name => join(destination, name));
      env.OBIT_INSTALL_SWAP_PATHS = paths.join('|');
      env.OBIT_INSTALL_SWAP_LOG = join(root, `${engine}-swaps.json`);
      let result;
      try { result = await run('remove', ['-Uninstall'], shell, destination); }
      finally { delete env.OBIT_INSTALL_SWAP_PATHS; }
      assert.equal(result.code, 0, result.stderr);
      const swaps = JSON.parse(await readFile(env.OBIT_INSTALL_SWAP_LOG, 'utf8'));
      assert.deepEqual(Object.values(swaps), [false, false], 'verified children could be replaced while uninstall was running');
    });
    await t.test(`${engine} TestReviewInstallerPreservesPreexistingPath`, async () => {
      const destination = join(root, `${engine}-preexisting-path`);
      env.OBIT_INSTALL_PATH_FIXTURE = join(root, `${engine}-user-path.txt`);
      const original = `C:\\Other;${destination};C:\\More`;
      await writeFile(env.OBIT_INSTALL_PATH_FIXTURE, original);
      let result = await run('good', ['-FixturePath'], shell, destination);
      assert.equal(result.code, 0, result.stderr);
      await copyFile(uninstaller, join(destination, 'obit.exe'));
      const receiptPath = join(destination, '.obituary-install.json');
      const receipt = JSON.parse(await readFile(receiptPath, 'utf8'));
      receipt.files.find(file => file.name === 'obit.exe').sha256 = sha256(await readFile(uninstaller));
      await writeFile(receiptPath, JSON.stringify(receipt));
      await writeFile(join(destination, 'other-tool.exe'), 'USER PROGRAM');
      result = await run('remove', ['-Uninstall', '-FixturePath'], shell, destination);
      assert.equal(result.code, 0, result.stderr);
      assert.equal(await readFile(env.OBIT_INSTALL_PATH_FIXTURE, 'utf8'), original, 'preexisting PATH was removed');
      assert.equal(await readFile(join(destination, 'other-tool.exe'), 'utf8'), 'USER PROGRAM');
    });
    await t.test(`${engine} TestReviewInstallerPartialDeleteLosesRetryExecutable`, async () => {
      const destination = join(root, `${engine}-uninstall-locked`);
      await prepareUninstall(destination, shell);
      const child = spawn(locker, [join(destination, 'obit-gui.exe'), 'deny-delete'], {env, windowsHide: true, stdio: ['pipe', 'pipe', 'pipe']});
      const exited = once(child, 'exit');
      try {
        await Promise.race([once(child.stdout, 'data'), exited.then(() => { throw new Error('locker exited'); })]);
        const result = await run('remove', ['-Uninstall'], shell, destination);
        assert.notEqual(result.code, 0);
        await stat(join(destination, 'obit.exe'));
        await stat(join(destination, '.obituary-install.json'));
      } finally { child.stdin.end(); await exited; }
      const retry = await run('remove', ['-Uninstall'], shell, destination);
      assert.equal(retry.code, 0, retry.stderr);
      await assert.rejects(stat(destination), {code: 'ENOENT'});
    });
    await t.test(`${engine} uninstall WhatIf keeps files and shortcut and never calls obit`, async () => {
      const destination = join(root, `${engine}-uninstall-dry`);
      await prepareUninstall(destination, shell);
      const callsBefore = await readFile(callLog).catch(error => { if (error.code === 'ENOENT') return null; throw error; });
      const result = await run('remove', ['-Uninstall', '-WhatIf'], shell, destination);
      assert.equal(result.code, 0, result.stderr);
      await stat(join(destination, 'obit.exe'));
      await stat(join(`${destination}-menu`, '讣告 Obituary.lnk'));
      assert.deepEqual(await readFile(callLog).catch(error => { if (error.code === 'ENOENT') return null; throw error; }), callsBefore);
    });
    await t.test(`${engine} uninstalls integrations before removing owned files and shortcut`, async () => {
      const destination = join(root, `${engine}-uninstall`);
      await prepareUninstall(destination, shell);
      const result = await run('remove', ['-Uninstall'], shell, destination);
      assert.equal(result.code, 0, result.stderr);
      assert.deepEqual(JSON.parse(await readFile(callLog, 'utf8')), ['uninstall', '--yes']);
      await assert.rejects(stat(destination), { code: 'ENOENT' });
      await assert.rejects(stat(join(`${destination}-menu`, '讣告 Obituary.lnk')), { code: 'ENOENT' });
    });
    await t.test(`${engine} preserves user files and modified installed files`, async () => {
      const destination = join(root, `${engine}-uninstall-preserve`);
      await prepareUninstall(destination, shell);
      await writeFile(join(destination, 'notes.txt'), 'USER NOTE');
      await writeFile(join(destination, 'README.txt'), 'EDITED BY USER');
      const result = await run('remove', ['-Uninstall'], shell, destination);
      assert.equal(result.code, 0, result.stderr);
      assert.match(result.stdout, /Preserving directory/);
      assert.equal(await readFile(join(destination, 'notes.txt'), 'utf8'), 'USER NOTE');
      assert.equal(await readFile(join(destination, 'README.txt'), 'utf8'), 'EDITED BY USER');
      assert.deepEqual((await readdir(destination)).sort(), ['README.txt', 'notes.txt']);
    });
    await t.test(`${engine} keeps the installation when integration cleanup fails`, async () => {
      const destination = join(root, `${engine}-uninstall-failure`);
      await prepareUninstall(destination, shell);
      env.OBIT_INSTALL_UNINSTALL_FAIL = '1';
      let result;
      try { result = await run('remove', ['-Uninstall'], shell, destination); }
      finally { delete env.OBIT_INSTALL_UNINSTALL_FAIL; }
      assert.notEqual(result.code, 0);
      assert.match(result.stderr, /obit uninstall failed/);
      await stat(join(destination, 'obit.exe'));
      await stat(join(`${destination}-menu`, '讣告 Obituary.lnk'));
    });
    await t.test(`${engine} preserves a shortcut changed by the user`, async () => {
      const destination = join(root, `${engine}-uninstall-shortcut`);
      await prepareUninstall(destination, shell);
      const target = join(root, 'user-application.exe');
      await shortcut(destination, shell, target);
      const result = await run('remove', ['-Uninstall'], shell, destination);
      assert.notEqual(result.code, 0);
      assert.match(result.stderr, /not owned by this installation/);
      assert.equal((await shortcut(destination, shell)).target.toLowerCase(), target.toLowerCase());
      await stat(join(destination, 'obit.exe'));
    });
  }
  await t.test('installer runs the real uninstall command with isolated OS boundaries', async () => {
    const original = resolve('internal/cli/uninstall/uninstall.go');
    const overlaySource = join(root, 'uninstall-boundaries.go');
    await writeFile(overlaySource, await readFile(original, 'utf8') + await readFile('scripts/testdata/install-boundaries.go.txt', 'utf8'));
    const overlay = join(root, 'uninstall-overlay.json');
    await writeFile(overlay, JSON.stringify({Replace: {[original]: overlaySource}}));
    const realCommand = join(root, 'real-uninstall.exe');
    await execute('go', ['build', '-overlay', overlay, '-o', realCommand, './scripts/testdata/install-command.go'], {env, windowsHide: true});
    const destination = join(root, 'real-command-install');
    await prepareUninstall(destination);
    await copyFile(realCommand, join(destination, 'obit.exe'));
    const receiptPath = join(destination, '.obituary-install.json');
    const receipt = JSON.parse(await readFile(receiptPath, 'utf8'));
    receipt.files.find(file => file.name === 'obit.exe').sha256 = sha256(await readFile(realCommand));
    await writeFile(receiptPath, JSON.stringify(receipt));
    // The real command inspects every supported agent. Override all registered
    // roots, including hosts whose normal default uses LOCALAPPDATA.
    for (const name of ['LOCALAPPDATA', 'QODER_CONFIG_DIR', 'QODERCN_CONFIG_DIR', 'VIBE_HOME',
      'KIMI_SHARE_DIR', 'DSH_HOME', 'COPILOT_HOME', 'GEMINI_CLI_HOME', 'FACTORY_HOME_OVERRIDE',
      'HERMES_HOME', 'CLINE_DIR', 'CLINE_DATA_DIR', 'KILO_CONFIG_DIR', 'OPENCODE_CONFIG_DIR',
      'OPENCLAW_STATE_DIR', 'OPENCLAW_HOME', 'PI_CODING_AGENT_DIR', 'XDG_CONFIG_HOME',
      'CODEBUDDY_CONFIG_DIR', 'MINIMAX_DATA_DIR', 'MAVIS_DATA_DIR', 'MCODE_INSTALL_DIR', 'ZCODE_DATA_BASE_DIR']) {
      env[name] = join(root, name);
      await mkdir(env[name], {recursive: true});
    }
    for (const name of Object.keys(env)) {
      if (name.startsWith('GIT_') || ['HERMES_DATA_DIR_SUFFIX', 'OPENCODE_CONFIG', 'OPENCLAW_CONFIG_PATH',
        'OPENCLAW_PROFILE', 'PI_CONFIG_DIR', 'OMP_PROFILE', 'PI_PROFILE'].includes(name)) delete env[name];
    }
    env.GIT_CEILING_DIRECTORIES = root;
    const rules = join(env.CLAUDE_CONFIG_DIR, 'CLAUDE.md');
    await writeFile(rules, '# USER RULES\n');
    const settings = join(env.CLAUDE_CONFIG_DIR, 'settings.json');
    await writeFile(settings, '{"theme":"dark"}\n');
    await execute(join(destination, 'obit.exe'), ['seed'], {env, windowsHide: true});
    const ledger = join(env.OBIT_HOME, 'store.json');
    const before = await readFile(ledger);
    const result = await run('remove', ['-Uninstall'], ps7, destination);
    assert.equal(result.code, 0, result.stderr);
    assert.match(result.stdout, /Uninstalled integrations and background services/);
    assert.equal(await readFile(rules, 'utf8'), '# USER RULES\n');
    await assert.rejects(stat(join(env.CLAUDE_CONFIG_DIR, 'OBIT.md')), {code: 'ENOENT'});
    assert.deepEqual(JSON.parse(await readFile(settings, 'utf8')), {theme: 'dark'});
    assert.deepEqual(await readFile(ledger), before);
    await assert.rejects(stat(destination), {code: 'ENOENT'});
  });
  await t.test('user environment registry values including PATH are unchanged', async () => {
    assert.deepEqual(await readUserEnvironment(), userEnvironmentBefore);
  });
  t.diagnostic(`Evidence: ${root}`);
});
