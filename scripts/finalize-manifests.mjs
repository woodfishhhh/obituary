import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

// GoReleaser v2.20.0 internal/pipe/scoop/scoop.go generates the architecture,
// binaries, URLs and hashes, but its Manifest type has no notes field.
// Keep that implementation and apply only our static bucket metadata here.
const dist = process.argv[2] ?? 'bin/release';
const target = join(dist, 'scoop', 'obituary.json');
const generated = JSON.parse(readFileSync(target, 'utf8'));
const template = JSON.parse(readFileSync(new URL('../packaging/scoop/obituary.json', import.meta.url), 'utf8'));
writeFileSync(target, JSON.stringify({
  ...template,
  version: generated.version,
  architecture: Object.fromEntries(Object.entries(generated.architecture).map(([arch, value]) =>
    [arch, { ...value, ...template.architecture[arch] }])),
}, null, 4) + '\n');
console.log(`Scoop manifest with uninstall notes: ${target}`);

// GoReleaser's Winget type also lacks ArchiveBinariesDependOnPath. The WinGet
// 1.9+ installer schema supports it: obit locates obitd relative to os.Executable,
// so expose the extracted directory on PATH rather than a WinGet Links symlink.
// Reference: microsoft/winget-pkgs doc/manifest/schema/1.10.0/installer.md.
const installerPath = join(dist, 'winget/manifests/w/woodfishhhh/Obituary', generated.version,
  'woodfishhhh.Obituary.installer.yaml');
const installer = readFileSync(installerPath, 'utf8');
if (!/^ArchiveBinariesDependOnPath:/m.test(installer)) {
  writeFileSync(installerPath, installer.replace(/^InstallerType: zip$/m,
    'InstallerType: zip\nArchiveBinariesDependOnPath: true'));
}
console.log(`WinGet manifest with colocated binaries on PATH: ${installerPath}`);
