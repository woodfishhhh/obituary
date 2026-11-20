// Geometry check for the README diagrams, run in a headless browser.
//
// Every <text> is measured the way the browser really draws it (ink box from
// canvas metrics, so descenders and side bearings are counted), then compared
// with the cards, chips, pills, arrows and icons around it. A file fails when
// text overlaps text, crosses a container edge, sits closer than the minimum
// padding or gap, touches an arrow, leaves the canvas, is drawn too small, has
// too little contrast, wraps into an orphan or a leading punctuation mark, or
// no longer spells the string from strings.json.
//
// It runs every file twice: with the fonts the SVG names, and with wide
// fallback fonts (Verdana and Courier New), which is what viewers without
// Segoe UI or SF Pro get. The second run only requires that nothing overlaps
// or crosses an edge; paddings and gaps are not enforced there.
//
// Containers are found by data-role (card, chip, pill, badge, arrow, icon,
// bg) when the file has it; a chip marked data-in="card" must lie wholly in a card; older files without it are recognised by shape.
//
// Usage: node scripts/check-readme-diagrams.mjs [--root <dir>] [--langs en,es] [--width 880] [--json] [--no-copy]
//   <dir> holds <lang>/{lifecycle,delete-guard}-{light,dark}.svg
//   (default docs/assets/readme/i18n). Exit code 1 when any check fails.
//   Needs the pinned Playwright: pnpm install --frozen-lockfile && pnpm browser, in docs/assets/readme/i18n.
import { createRequire } from "node:module";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const repo = path.resolve(here, "..");
const args = process.argv.slice(2);
const opt = (name, fallback) => {
  const i = args.indexOf(`--${name}`);
  return i >= 0 ? args[i + 1] : fallback;
};
const root = path.resolve(opt("root", path.join(repo, "docs/assets/readme/i18n")));
const displayWidth = Number(opt("width", "880"));
const asJson = args.includes("--json");
const skipCopy = args.includes("--no-copy");

const ALL_LANGS = ["en", "zh-CN", "zh-TW", "ja", "ko", "es", "fr", "de", "pt-BR", "ru"];
const LANGS = opt("langs", "") ? opt("langs", "").split(",") : ALL_LANGS;
const DIAGRAMS = ["lifecycle", "delete-guard"];
const THEMES = ["light", "dark"];
const NO_LINE_START = new Set(Array.from(",.!?;:)]}，。！？、；：）」』》”’…"));
const NO_SPACE_JOIN = new Set(["zh-CN", "zh-TW", "ja"]);

// Minimums, in SVG units unless noted.
const LIMITS = {
  cardPad: 12, // text to card edge
  chipPadX: 8, // text to chip/pill edge, sideways
  chipPadY: 3,
  gap: 8, // between a chip/pill/icon and the text next to it
  textGap: 6, // between two texts on one line
  arrowClear: 8, // text to an arrow
  edge: 10, // anything to the canvas edge
  minFont: 14, // displayed px at --width
  minContrast: 4.5,
};

function loadPlaywright() {
  const pkg = path.join(repo, "docs/assets/readme/i18n/package.json");
  try {
    return createRequire(pkg)("playwright");
  } catch {
    console.error("playwright is not installed: run `pnpm install --frozen-lockfile && pnpm browser` in " + path.dirname(pkg));
    process.exit(2);
  }
}

// Runs in the page: returns every text and shape with its box in SVG units.
function extract(profile) {
  const svg = document.querySelector("svg");
  const vb = svg.viewBox.baseVal;
  if (profile === "wide") {
    const style = document.createElement("style");
    style.textContent =
      `text{font-family:Verdana,'Microsoft YaHei','Malgun Gothic','Yu Gothic',sans-serif !important}` +
      `text[font-family*="monospace"]{font-family:'Courier New',monospace !important}`;
    document.head.appendChild(style);
  }
  const origin = svg.getBoundingClientRect();
  const ctx = document.createElement("canvas").getContext("2d");
  const rel = (r) => ({ x0: r.left - origin.left, y0: r.top - origin.top, x1: r.right - origin.left, y1: r.bottom - origin.top });
  const texts = [];
  for (const el of svg.querySelectorAll("text")) {
    const cs = getComputedStyle(el);
    const size = parseFloat(cs.fontSize);
    ctx.font = `${cs.fontWeight} ${size}px ${cs.fontFamily}`;
    const value = el.textContent;
    const m = ctx.measureText(value);
    const r = rel(el.getBoundingClientRect());
    const baseline = el.y.baseVal[0].value;
    texts.push({
      value, size, weight: cs.fontWeight, fill: el.getAttribute("fill"), x: el.x.baseVal[0].value, baseline,
      key: el.dataset.k || "", group: el.dataset.g || "", kind: el.dataset.t || "",
      mono: (el.getAttribute("font-family") || "").includes("monospace"),
      box: { x0: r.x0, x1: r.x1, y0: baseline - m.actualBoundingBoxAscent, y1: baseline + m.actualBoundingBoxDescent },
    });
  }
  const shapes = [];
  const all = Array.from(svg.querySelectorAll("rect, path, circle, polygon, line"));
  all.forEach((el, index) => {
    const r = rel(el.getBoundingClientRect());
    const stroke = parseFloat(el.getAttribute("stroke-width") || "0") * (el.getAttribute("stroke") ? 0.5 : 0);
    shapes.push({
      index, tag: el.tagName, role: el.dataset.role || "", in: el.dataset.in || "", fill: el.getAttribute("fill"), stroke: el.getAttribute("stroke"),
      strokeWidth: parseFloat(el.getAttribute("stroke-width") || "0"), rx: parseFloat(el.getAttribute("rx") || "0"), id: el.id || "",
      box: { x0: r.x0 - stroke, y0: r.y0 - stroke, x1: r.x1 + stroke, y1: r.y1 + stroke },
    });
  });
  return { vb: [vb.width, vb.height], texts, shapes };
}

const area = (b) => Math.max(0, b.x1 - b.x0) * Math.max(0, b.y1 - b.y0);
const inter = (a, b) => area({ x0: Math.max(a.x0, b.x0), y0: Math.max(a.y0, b.y0), x1: Math.min(a.x1, b.x1), y1: Math.min(a.y1, b.y1) });
const inside = (inner, outer, tol = 0.5) => inner.x0 >= outer.x0 - tol && inner.x1 <= outer.x1 + tol && inner.y0 >= outer.y0 - tol && inner.y1 <= outer.y1 + tol;
const hit = (a, b, tol = 0.5) => a.x0 < b.x1 - tol && b.x0 < a.x1 - tol && a.y0 < b.y1 - tol && b.y0 < a.y1 - tol;
const vOverlap = (a, b) => Math.min(a.y1, b.y1) - Math.max(a.y0, b.y0) > 1;
const grow = (b, d) => ({ x0: b.x0 - d, y0: b.y0 - d, x1: b.x1 + d, y1: b.y1 + d });

function rgb(color) {
  const m = /^#([0-9a-f]{6})$/i.exec(color || "");
  if (!m) return null;
  const n = parseInt(m[1], 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}
function luminance(c) {
  const f = (v) => ((v /= 255) <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4);
  return 0.2126 * f(c[0]) + 0.7152 * f(c[1]) + 0.0722 * f(c[2]);
}
function contrast(a, b) {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

// Files without data-role are classified by shape, so the check also runs on
// the diagrams the first generator produced.
function classify(shapes, vb) {
  for (const s of shapes) {
    if (s.role) continue;
    const w = s.box.x1 - s.box.x0;
    const h = s.box.y1 - s.box.y0;
    if (s.tag === "rect" && w >= vb[0] - 6 && h >= vb[1] - 6) s.role = "bg";
    else if (s.tag === "rect" && s.rx > 0 && h <= 48) s.role = "chip";
    else if (s.tag === "path" && s.strokeWidth === 3 && s.fill === "none") s.role = "arrow";
    else if (s.tag === "path" && w >= 150 && h >= 100) s.role = "card";
    else if (s.id.includes("star")) s.role = "icon";
  }
  return shapes;
}

function groupsOf(texts) {
  const groups = [];
  for (const t of texts) {
    const last = groups[groups.length - 1];
    const prev = last && last[last.length - 1];
    const same = t.group
      ? prev && prev.group === t.group
      : prev && !prev.group && Math.abs(prev.x - t.x) < 0.5 && prev.size === t.size && prev.fill === t.fill &&
        t.baseline - prev.baseline > prev.size * 1.1 && t.baseline - prev.baseline < prev.size * 1.7 && t.baseline > prev.baseline;
    if (same) last.push(t);
    else groups.push([t]);
  }
  return groups;
}

function checkFile(data, ctx) {
  const { lang, strict } = ctx;
  const found = [];
  const add = (rule, message) => found.push({ rule, message });
  const { texts, vb } = data;
  const shapes = classify(data.shapes, vb);
  const bg = shapes.find((s) => s.role === "bg");
  const bgFill = rgb(bg && bg.fill) || [255, 255, 255];
  const of = (role) => shapes.filter((s) => s.role === role);
  const cards = of("card");
  const chips = [...of("chip"), ...of("pill"), ...of("badge")];
  const arrows = of("arrow");
  const icons = of("icon");
  const label = (t) => JSON.stringify(t.value.length > 28 ? t.value.slice(0, 28) + "…" : t.value);
  const pad = (v) => (strict ? v : 0);

  texts.forEach((a, i) => {
    for (const b of texts.slice(i + 1)) {
      if (hit(a.box, b.box)) add("text-overlap", `${label(a)} overlaps ${label(b)}`);
      else if (strict && vOverlap(a.box, b.box)) {
        const gap = Math.max(b.box.x0 - a.box.x1, a.box.x0 - b.box.x1);
        if (gap >= 0 && gap < LIMITS.textGap) add("text-gap", `${label(a)} and ${label(b)} are ${gap.toFixed(1)}px apart`);
      }
    }
  });

  for (const t of texts) {
    const margin = strict ? LIMITS.edge : 0;
    if (t.box.x0 < margin || t.box.y0 < margin || t.box.x1 > vb[0] - margin || t.box.y1 > vb[1] - margin) {
      add("outside-canvas", `${label(t)} is closer than ${margin}px to the canvas edge or outside it`);
    }
    for (const c of chips) {
      if (hit(t.box, c.box) && !inside(t.box, c.box)) add("text-crosses-chip", `${label(t)} runs over the edge of ${c.role} at ${Math.round(c.box.x0)},${Math.round(c.box.y0)}`);
      else if (inside(t.box, c.box) && strict) {
        const px = Math.min(t.box.x0 - c.box.x0, c.box.x1 - t.box.x1);
        const py = Math.min(t.box.y0 - c.box.y0, c.box.y1 - t.box.y1);
        if (px < LIMITS.chipPadX || py < LIMITS.chipPadY) add("chip-padding", `${label(t)} has ${Math.min(px, py).toFixed(1)}px inside its ${c.role}`);
      } else if (strict && !hit(t.box, c.box) && vOverlap(t.box, c.box)) {
        const gap = Math.max(c.box.x0 - t.box.x1, t.box.x0 - c.box.x1);
        if (gap >= 0 && gap < LIMITS.gap) add("chip-gap", `${label(t)} is ${gap.toFixed(1)}px from the ${c.role} next to it`);
      }
    }
    for (const c of cards) {
      if (hit(t.box, c.box) && !inside(t.box, c.box)) add("text-crosses-card", `${label(t)} runs over the edge of the card at ${Math.round(c.box.x0)},${Math.round(c.box.y0)}`);
      else if (inside(t.box, c.box) && strict) {
        const p = Math.min(t.box.x0 - c.box.x0, c.box.x1 - t.box.x1, t.box.y0 - c.box.y0, c.box.y1 - t.box.y1);
        if (p < LIMITS.cardPad) add("card-padding", `${label(t)} has ${p.toFixed(1)}px inside its card`);
      }
    }
    for (const a of arrows) {
      if (hit(grow(a.box, pad(LIMITS.arrowClear)), t.box)) add("text-on-arrow", `${label(t)} touches the arrow at ${Math.round(a.box.x0)},${Math.round(a.box.y0)}`);
    }
    for (const ic of icons) {
      if (hit(grow(ic.box, pad(4)), t.box)) add("text-on-icon", `${label(t)} touches the icon ${ic.id || ""}`);
    }
    // text in no container must not sit on a card or chip border either way round
    const home = [...chips, ...cards].filter((c) => inside(t.box, c.box)).sort((p, q) => area(p.box) - area(q.box))[0];
    const under = home ? rgb(home.fill) || bgFill : bgFill;
    const ink = rgb(t.fill);
    if (ink && strict) {
      const ratio = contrast(ink, under);
      if (ratio < LIMITS.minContrast) add("contrast", `${label(t)} contrast ${ratio.toFixed(2)} < ${LIMITS.minContrast}`);
    }
    const shown = (t.size * displayWidth) / vb[0];
    if (strict && shown < LIMITS.minFont) add("font-size", `${label(t)} is drawn at ${shown.toFixed(1)}px at ${displayWidth}px width`);
  }

  if (strict) {
    // Containers: chips and cards must nest cleanly or stay apart.
    const boxes = [...cards, ...chips];
    boxes.forEach((a, i) => {
      for (const b of boxes.slice(i + 1)) {
        if (hit(a.box, b.box) && !inside(a.box, b.box) && !inside(b.box, a.box)) add("shape-overlap", `${a.role} at ${Math.round(a.box.x0)},${Math.round(a.box.y0)} overlaps ${b.role} at ${Math.round(b.box.x0)},${Math.round(b.box.y0)}`);
      }
    });
    for (const s of [...cards, ...chips, ...arrows]) {
      if (!inside(s.box, grow({ x0: 0, y0: 0, x1: vb[0], y1: vb[1] }, -4))) add("outside-canvas", `${s.role} at ${Math.round(s.box.x0)},${Math.round(s.box.y0)} leaves the canvas`);
    }
    // A command chip that belongs to a card (data-in="card") sits wholly inside one.
    for (const c of chips.filter((s) => s.in === "card")) {
      if (!cards.some((k) => inside(c.box, k.box))) add("chip-outside-card", `command chip at ${Math.round(c.box.x0)},${Math.round(c.box.y0)} is not wholly inside a card`);
    }
    for (const c of chips) {
      for (const a of arrows) if (hit(c.box, a.box)) add("shape-overlap", `${c.role} at ${Math.round(c.box.x0)},${Math.round(c.box.y0)} sits on an arrow`);
    }
  }

  for (const g of groupsOf(texts)) {
    if (g.length < 2) continue;
    const last = g[g.length - 1].value;
    if (last.replace(/\s/g, "").length <= 2) add("orphan-line", `last line ${JSON.stringify(last)} is one or two characters`);
    for (const t of g.slice(1)) if (NO_LINE_START.has(t.value[0])) add("line-start-punctuation", `line ${label(t)} starts with punctuation`);
  }
  return found;
}

// The wrapped lines of every labelled text must read exactly like strings.json.
function checkCopy(data, copy, lang, diagram) {
  const found = [];
  const joiner = NO_SPACE_JOIN.has(lang) ? "" : " ";
  const byKey = new Map();
  for (const t of data.texts) if (t.key) byKey.set(t.key, [...(byKey.get(t.key) || []), t.value]);
  const required =
    diagram === "lifecycle"
      ? ["title", "register", "register_note", "expire", "expire_note", "reap", "bury", "bury_note", "resurrect", "resurrect_note", "purge", "purge_note", "before", "extend_note", "spare_note"]
      : ["guard_title", "guard_first", "guard_bridge", "guard_equiv", "guard_second", "guard_res", "guard_res_note"];
  const norm = (s) => s.replace(/\s+/g, " ").trim();
  for (const key of required) {
    const lines = byKey.get(key);
    if (!lines) {
      found.push({ rule: "copy-missing", message: `${key} is not in the drawing` });
      continue;
    }
    const drawn = norm(lines.map(norm).join(joiner));
    if (drawn !== norm(copy[key])) found.push({ rule: "copy-differs", message: `${key} reads ${JSON.stringify(drawn)}, strings.json has ${JSON.stringify(copy[key])}` });
  }
  return found;
}

const strings = JSON.parse(fs.readFileSync(path.join(repo, "docs/assets/readme/i18n/strings.json"), "utf8"));
const { chromium } = loadPlaywright();
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage({ viewport: { width: 1200, height: 900 } });
const results = [];
for (const lang of LANGS) {
  for (const diagram of DIAGRAMS) {
    for (const theme of THEMES) {
      const file = path.join(root, lang, `${diagram}-${theme}.svg`);
      if (!fs.existsSync(file)) {
        results.push({ file: `${lang}/${diagram}-${theme}`, issues: [{ rule: "missing", message: "file does not exist" }] });
        continue;
      }
      const svg = fs.readFileSync(file, "utf8");
      const issues = [];
      let measured = null;
      for (const profile of ["native", "wide"]) {
        await page.setContent(`<!doctype html><body style="margin:0">${svg}</body>`);
        const data = await page.evaluate(extract, profile);
        if (profile === "native") measured = data;
        const found = checkFile(data, { lang, strict: profile === "native" });
        for (const f of found) issues.push({ ...f, profile });
      }
      for (const f of skipCopy ? [] : checkCopy(measured, strings[lang], lang, diagram)) issues.push({ ...f, profile: "native" });
      if (/textLength=|lengthAdjust=/.test(svg)) issues.push({ rule: "text-compression", message: "textLength/lengthAdjust squeezes text", profile: "native" });
      results.push({ file: `${lang}/${diagram}-${theme}`, size: measured.vb, issues });
    }
  }
}
await browser.close();

const failed = results.filter((r) => r.issues.length);
if (asJson) {
  console.log(JSON.stringify(results, null, 2));
} else {
  for (const r of failed) {
    console.log(`\n${r.file}  (${r.issues.length})`);
    const seen = new Set();
    for (const i of r.issues) {
      const line = `  [${i.profile}] ${i.rule}: ${i.message}`;
      if (!seen.has(line)) console.log(line), seen.add(line);
    }
  }
  const counts = {};
  for (const r of failed) for (const i of r.issues) counts[i.rule] = (counts[i.rule] || 0) + 1;
  console.log(`\n${results.length - failed.length}/${results.length} files pass` + (failed.length ? `; findings by rule: ${JSON.stringify(counts)}` : ""));
}
process.exit(failed.length ? 1 : 0);
