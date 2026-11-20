// What the landing page says in every language that is not a sentence: commands, release files
// and the real outputs the story section quotes. Each language's own sentences are in ./<lang>.ts.

import { version } from "@/config";

export type Os = "windows" | "macos" | "linux";

export const repo = "https://github.com/woodfishhhh/obituary";
export const qqGroup = "1135603207";

// Commands are product text and stay identical in both languages; the install
// lines mirror docs/install.md.
export type Method = "scoop" | "winget" | "brew" | "script";
const shellScript = [
  "curl -fsSL https://raw.githubusercontent.com/woodfishhhh/obituary/main/install.sh -o install.sh",
  "bash install.sh",
];
export const installCode: Record<Os, Partial<Record<Method, readonly string[]>>> = {
  windows: {
    scoop: ["scoop bucket add obituary https://github.com/woodfishhhh/scoop-bucket", "scoop install obituary"],
    winget: ["winget install woodfishhhh.Obituary"],
    script: [
      "irm https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1 -OutFile install.ps1",
      ".\\install.ps1",
    ],
  },
  macos: { brew: ["brew install --cask woodfishhhh/tap/obituary"], script: shellScript },
  linux: { script: shellScript },
};

// Release archives per platform, each with the checksums; the macOS labels
// are set per language.
const archive = (platform: string, ext: string) => `obituary_${version}_${platform}.${ext}`;
const checksums = { label: "checksums.txt", file: "checksums.txt" };
export const archives = (macLabels: [string, string]): Record<Os, { label: string; file: string }[]> => ({
  windows: [
    { label: "x64", file: archive("windows_amd64", "zip") },
    { label: "ARM64", file: archive("windows_arm64", "zip") },
    checksums,
  ],
  macos: [
    { label: macLabels[0], file: archive("darwin_arm64", "tar.gz") },
    { label: macLabels[1], file: archive("darwin_amd64", "tar.gz") },
    checksums,
  ],
  linux: [
    { label: "x64", file: archive("linux_amd64", "tar.gz") },
    { label: "ARM64", file: archive("linux_arm64", "tar.gz") },
    checksums,
  ],
});

// The sentence a user sends to their agent. README.md keeps it in English in every language, so
// every page does too; only the Simplified Chinese page has its own wording.
export const agentSentenceEn = "Read https://obituary.woodfish.site/agent-install.md and follow it to install Obituary and connect it to yourself.";

// One real run with the delete guard on: the reason the hook gave Claude Code
// when it denied `rm -rf build` (without its JSON envelope), and the commands
// that followed, output as printed.
export const guardRun = [
  {
    cmd: "rm -rf build",
    out: "Obituary's delete guard is on: `'C:/work/obit.exe' rm -rf build` provides restorable deletion; `obit resurrect` restores graveyard entries.",
    deny: true,
  },
  { cmd: "obit rm -rf build", out: "" },
  { cmd: "obit graveyard --here", out: "ID  BURIED            PURGE  SIZE  PATH\n#2  2026-11-11 00:36  in 7d  23 B  C:\\work\\build" },
  { cmd: "obit resurrect --last", out: "#2 resurrected  C:\\work\\build" },
] as const;

// A real run in Git Bash: with OUT empty, the shell turns "/" into Git's
// install folder, and obit rm refuses it.
export const emptyVarRun = {
  cmd: 'OUT=; obit rm -rf "$OUT"/',
  out: "rm: it is dangerous to operate recursively on 'C:/Program Files/Git/' (same as '/')",
};

// The grave notice an agent receives, in the hook's own wording; the path,
// number and dates are sample values.
export const graveNotice =
  'Obituary: "debug.log" was buried as #5 (buried 2026-11-11T09:20:00+08:00, restorable until 2026-11-18T09:20:00+08:00). `obit resurrect 5` restores it.';


export type LookId = "calendar" | "palette";
export type PageId = "upcoming" | "graveyard" | "decisions";
