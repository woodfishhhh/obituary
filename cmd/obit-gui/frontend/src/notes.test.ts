import assert from "node:assert/strict";
import { test } from "node:test";
import { notesFor } from "./notes.ts";

const zh = ["# 讣告 1.6.0", "", "- **Linux 小窗口（预览版）**：Ubuntu 24.04（amd64）可用 `.deb` 安装。", "- Windows 安装脚本现在直接校验 SHA-256 和解压安装包。"].join("\n");
const en = ["# Obituary 1.6.0", "", "- **Linux desktop window (preview)**: install the `.deb` on Ubuntu 24.04 (amd64).", "- The Windows installer now verifies SHA-256 and extracts the archive."].join("\n");

test("a bilingual release shows only the window's language", () => {
  const both = `${zh}\n\n---\n\n${en}\n`;
  assert.equal(notesFor(both, "zh"), zh);
  assert.equal(notesFor(both, "en"), en);
});

test("the order of the two parts does not matter", () => {
  const both = `${en}\n\n---\n\n${zh}`;
  assert.equal(notesFor(both, "zh"), zh);
  assert.equal(notesFor(both, "en"), en);
});

test("the rule never reaches the window, whatever it looks like", () => {
  for (const hr of ["---", "***", "___", "- - -", "-----", "  ---  "]) {
    const out = notesFor(`${zh}\n${hr}\n${en}`, "zh");
    assert.equal(out, zh, hr);
  }
});

test("a window language the notes lack gets English", () => {
  assert.equal(notesFor(`${en}\n\n---\n\n${en}`, "zh"), `${en}\n\n${en}`);
  assert.equal(notesFor(`# Notes\n\n---\n\n${en}`, "zh"), `# Notes\n\n${en}`);
});

test("when nothing is English, a Chinese part is better than nothing", () => {
  assert.equal(notesFor(`${zh}\n\n---\n\n${zh}`, "en"), `${zh}\n\n${zh}`);
});

test("notes in one language pass through unchanged", () => {
  assert.equal(notesFor(zh, "en"), zh);
  assert.equal(notesFor(en, "zh"), en);
  assert.equal(notesFor("", "zh"), "");
  assert.equal(notesFor("  \n", "en"), "");
});

test("a table separator or a dash inside text is not a rule", () => {
  const table = ["| a | b |", "|---|---|", "| 1 | 2 |"].join("\n");
  assert.equal(notesFor(`${en}\n\n${table}`, "en"), `${en}\n\n${table}`);
  assert.equal(notesFor("- one\n- two --- three", "en"), "- one\n- two --- three");
});

test("a few Han characters in English notes do not make them Chinese", () => {
  const named = `# Obituary 1.6.0 (讣告)\n\n- ${"A long English sentence about the release. ".repeat(4)}`;
  assert.equal(notesFor(`${zh}\n\n---\n\n${named}`, "en"), named.trim());
});

test("Chinese notes heavy on Latin names are still Chinese", () => {
  const heavy = "修复 obit update 的 --apply 参数，obitd 和 obit-gui 一并升级。";
  assert.equal(notesFor(`${heavy}\n\n---\n\n${en}`, "zh"), heavy);
});

test("CRLF line endings work the same", () => {
  const both = `${zh}\r\n\r\n---\r\n\r\n${en}\r\n`.replace(/\n/g, "\r\n").replace(/\r\r\n/g, "\r\n");
  assert.equal(notesFor(both, "en").replace(/\r/g, ""), en);
});
