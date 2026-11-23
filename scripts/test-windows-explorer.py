"""Exercise registered classic-menu handlers on a disposable Windows runner, not Explorer UI."""
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import time
import traceback
import urllib.request
import zipfile

if sys.platform != 'win32' or os.getenv('RUNNER_ENVIRONMENT') != 'github-hosted' or os.getenv('GITHUB_ACTIONS') != 'true':
    sys.exit('requires a disposable GitHub-hosted Windows runner')

import winreg

temp = Path(os.environ['RUNNER_TEMP']).resolve()
work = Path(tempfile.mkdtemp(dir=temp, prefix='obit-menu-'))
evidence = temp / 'obit-explorer-acceptance'
evidence.mkdir()
cli, daemon, client = [work / name for name in ('obit.exe', 'obitd.exe', 'drop.exe')]
version = os.environ['EXPECTED_VERSION']
assert re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+', version)
failures = []


def record(name, value):
    (evidence / (name + '.json')).write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding='utf-8')
    print(name, value, flush=True)


def value(key, field=''):
    with winreg.OpenKey(winreg.HKEY_CURRENT_USER, 'Software\\Classes\\' + key) as k:
        return winreg.QueryValueEx(k, field)[0]


def command(args):
    for name in ('OBIT_HOME', 'OBIT_CONFIG_DIR', 'OBIT_USER_HOME'):
        assert Path(os.environ[name]).resolve().is_relative_to(work), name
    result = subprocess.run(args, capture_output=True, encoding='utf-8', errors='replace', timeout=90, cwd=work)
    if result.returncode:
        raise RuntimeError((args, result.returncode, result.stdout, result.stderr))
    return result.stdout


def attrs(paths):
    return {p.name: p.stat().st_file_attributes for p in paths}


def registration(language):
    verbs = {}
    for kind in ('*', 'Directory'):
        for verb in ('Mark', 'Bury', 'Resurrect'):
            key = kind + '\\shell\\Obituary.' + verb
            verbs[key] = (value(key, 'MUIVerb'), value(key, 'MultiSelectModel'))
        assert value(kind + r'\shell\Obituary.Mark', 'ExtendedSubCommandsKey') == 'Obituary.ContextMenu'
    policy = r'Directory\Background\shell\Obituary.Policy'
    verbs[policy] = value(policy, 'MUIVerb')
    shared = r'Obituary.ContextMenu\shell'
    durations = [value(shared + '\\' + str(i).zfill(2), 'MUIVerb') for i in (1, 2, 3)]
    record(language + '-registry', dict(verbs=verbs, durations=durations))
    assert durations == (['1 天', '7 天', '30 天'] if language == 'zh-CN' else ['1 day', '7 days', '30 days'])
    for verb in ('Mark', 'Bury', 'Resurrect'):
        assert verbs['*\\shell\\Obituary.' + verb] == verbs['Directory\\shell\\Obituary.' + verb]
    return shared, policy


def exercise(language):
    root = work / language
    for key, leaf in [('OBIT_HOME', 'data'), ('OBIT_CONFIG_DIR', 'config'), ('OBIT_USER_HOME', 'user')]:
        path = root / leaf
        path.mkdir(parents=True)
        os.environ[key] = str(path)
    os.environ['OBIT_LANG'] = language
    config = root / 'config' / 'config.toml'
    config.write_text(f'[ui]\nlanguage="{language}"\n[explorer]\neverywhere=true\n[update]\ncheck=false\n')
    samples = root / 'samples'
    folder = samples / 'test folder'
    folder.mkdir(parents=True)
    file = samples / 'test file.txt'
    ini = samples / 'desktop.ini'
    ini.write_bytes(b'; preserve this comment\r\n[.ShellClassInfo]\r\nInfoTip=original\r\n')
    original = ini.read_bytes()
    file.write_bytes(b'unchanged')
    (folder / 'inside').write_bytes(b'folder contents')
    paths = [file, folder, samples, ini]
    before = attrs(paths)
    foreign = r'*\shell\Acceptance.Foreign'
    with winreg.CreateKey(winreg.HKEY_CURRENT_USER, 'Software\\Classes\\' + foreign) as k:
        winreg.SetValueEx(k, 'MUIVerb', 0, winreg.REG_SZ, 'Third party sample')
    init = command([cli, 'init', '-g', '--yes', '--no-shortcuts', '--no-skill'])
    (evidence / f'{language}-init.txt').write_text(init, encoding='utf-8')
    shared, policy = registration(language)

    def drop(key, action, selection):
        clsid = value(key + r'\DropTarget', 'CLSID')
        registered = value('CLSID\\' + clsid + r'\LocalServer32')
        # Shell would launch this exact server; start it here so it inherits the test home.
        normalized = registered.replace('"', '').casefold()
        assert normalized == f'{daemon} --context-menu-server {cli} {action}'.casefold(), registered
        with subprocess.Popen(registered, cwd=work, stdout=subprocess.PIPE, stderr=subprocess.PIPE) as server:
            try:
                time.sleep(1)
                command([client, clsid, samples, *selection])
                stdout, stderr = server.communicate(timeout=80)
            except (OSError, RuntimeError, subprocess.SubprocessError) as exc:
                failures.append(f'{language} {action}: {exc}')
                server.kill()
                server.wait()
                # Report handler-only coverage separately if COM transport failed before dispatch.
                if action != 'bury' or all(p.exists() for p in selection):
                    command([daemon, '--context-menu', cli, action, *selection])
                record(language + '-' + action + '-fallback', failures[-1])
                return
            finally:
                if server.poll() is None:
                    server.kill()
            receipt = (root / 'data/context-menu/last-result.txt').read_text(encoding='utf-8')
            record(language + '-' + action, dict(command=registered, exit=server.returncode, receipt=receipt,
                                                 stderr=stderr.decode('utf-8', 'replace')))
            if server.returncode:
                failures.append(f'{language} {action}: registered server returned {server.returncode}: {stderr!r}')
            assert all(str(p) in receipt for p in selection), (stdout, stderr)

    def direct(key, action, placeholder, path):
        registered = value(key + r'\command')
        assert registered.replace('"', '').casefold() == f'{daemon} --context-menu {cli} {action} {placeholder}'.casefold()
        actual = registered.replace(placeholder, str(path) + ('\\.' if placeholder == '%V\\.' else ''))
        try:
            command(actual)
        except RuntimeError as exc:
            failures.append(f'{language} {action}: {exc}')
        record(language + '-' + action, actual)

    for index, days, selected in [(1, 1, [file]), (2, 7, [folder]), (3, 30, [file, folder])]:
        start = time.time()
        drop(shared + '\\' + str(index).zfill(2), f'{days}d', selected)
        rows = {r['path']: r for r in json.loads(command([cli, 'list', '--json']))}
        for p in selected:
            expiry = dt.datetime.fromisoformat(rows[str(p)]['expiresAt'].replace('Z', '+00:00')).timestamp()
            assert start - 2 <= expiry - days * 86400 <= time.time() + 2 and p.exists(), rows
        record(f'{language}-list-{days}', rows)

    command([cli, 'spare', '--', str(folder)])
    assert str(folder) not in {row['path'] for row in json.loads(command([cli, 'list', '--json']))}
    drop(r'Directory\shell\Obituary.Bury', 'bury', [folder])
    assert not folder.exists() and file.read_bytes() == b'unchanged'
    graves = json.loads(command([cli, 'graveyard', '--json']))
    assert [row['path'] for row in graves['burials']] == [str(folder)], graves
    direct(r'*\shell\Obituary.Resurrect', 'resurrect', '%1', file)
    assert (folder / 'inside').read_bytes() == b'folder contents' and not json.loads(command([cli, 'graveyard', '--json']))['burials']
    direct(policy, 'policy', '%V\\.', samples)
    watches = json.loads(command([cli, 'watch', '--list', '--json']))
    record(language + '-policy-proposals', watches)
    assert any(w['path'] == str(samples) and w['consent'] == 'pending' for w in watches), watches
    labelled = ini.read_bytes().decode('utf-16')
    expiry = dt.datetime.fromisoformat(rows[str(file)]['expiresAt'].replace('Z', '+00:00')).astimezone()
    day = expiry.strftime('%m-%d')
    label = f'{file.stem}（{day} 下葬）' if language == 'zh-CN' else f'{file.stem} ({day} burial)'
    assert f'{file.name}={label}' in labelled.splitlines() and 'InfoTip=original' in labelled, labelled
    during = attrs(paths)
    output = command([cli, 'uninstall', '--yes'])
    (evidence / f'{language}-uninstall.txt').write_text(output, encoding='utf-8')
    after = attrs(paths)
    record(language + '-cleanup', dict(before=before, labelled=during, after=after,
                                       ini_restored=ini.read_bytes() == original, foreign=value(foreign, 'MUIVerb')))
    assert ini.read_bytes() == original and before == after and file.read_bytes() == b'unchanged'
    assert value(foreign, 'MUIVerb') == 'Third party sample'
    try:
        value(r'*\shell\Obituary.Mark', 'MUIVerb')
    except FileNotFoundError:
        return
    raise AssertionError('owned menu survived uninstall')


archive = f'obituary_{version}_windows_amd64.zip'
base = f'https://github.com/woodfishhhh/obituary/releases/download/v{version}/'
for name in (archive, 'checksums.txt'):
    urllib.request.urlretrieve(base + name, work / name)
expected, = [line.split()[0] for line in (work / 'checksums.txt').read_text().splitlines() if line.split()[-1] == archive]
digest = hashlib.sha256((work / archive).read_bytes()).hexdigest()
assert digest == expected
record('release', dict(archive=archive, sha256=digest))
with zipfile.ZipFile(work / archive) as z:
    for name in ('obit.exe', 'obitd.exe', 'obit-gui.exe'):
        (work / name).write_bytes(z.read(name))
try:
    subprocess.run(['g++', '-std=c++17', '-municode', str(Path(__file__).with_name('explorer-drop.cpp')),
                    '-o', client, '-lole32', '-lshell32', '-luuid'], check=True)
except (OSError, subprocess.CalledProcessError) as error:
    failures.append(str(error))
for language in ('zh-CN', 'en'):
    try:
        exercise(language)
    except Exception:
        failures.append(traceback.format_exc())
record('failures', failures)
if failures:
    sys.exit(1)
