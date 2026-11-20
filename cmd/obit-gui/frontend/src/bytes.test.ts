import assert from "node:assert/strict";
import { test } from "node:test";
import { bytes } from "./bytes.ts";

const KB = 1024;
const MB = KB * 1024;
const GB = MB * 1024;

test("bytes up to MB keep the one-decimal form", () => {
  assert.equal(bytes(0), "0 B");
  assert.equal(bytes(512), "512 B");
  assert.equal(bytes(1536), "1.5 KB");
  assert.equal(bytes(120 * KB), "120 KB");
  assert.equal(bytes(1.8 * MB), "1.8 MB");
  assert.equal(bytes(412 * MB), "412 MB");
});

test("bytes from GB up show two decimals", () => {
  assert.equal(bytes(165.36 * GB), "165.36 GB");
  assert.equal(bytes(1.3 * GB), "1.30 GB");
  assert.equal(bytes(2.4 * GB), "2.40 GB");
  assert.equal(bytes(1023.99 * MB), "1024 MB");
  assert.equal(bytes(3 * 1024 * GB), "3.00 TB");
});
