// Layout engine for the README diagrams. generate.mjs injects this file into a
// headless browser page and calls window.buildAll(strings): every width below is
// measured with the browser's own text engine, in our font stack and in a wide
// fallback stack (what viewers without Segoe UI or SF Pro get), and the larger
// of the two decides. Nothing here guesses a glyph width.
//
// Both diagrams are one vertical timeline on a fixed 960-unit canvas (drawn at
// about 0.92x on GitHub, so notes land near 16px): a numbered rail, one wide
// card per step, and the words that belong to an arrow (the "Reap" pill, the
// "before it expires" actions) set beside the arrow in the gap between cards,
// where nothing else lives. Long labels wrap with a balanced line breaker that
// never splits a word, never starts a line with closing punctuation and never
// leaves an orphan.
(() => {
  const W = 960, M = 32, CW = W - 2 * M, PAD = 22, BADGE = 34, CX = M + PAD + BADGE / 2;
  const SANS = "-apple-system, BlinkMacSystemFont, 'Segoe UI', 'Noto Sans', 'Noto Sans CJK SC', 'Noto Sans CJK TC', 'Noto Sans JP', 'Noto Sans KR', 'Noto Sans Cyrillic', 'Microsoft YaHei', 'PingFang SC', 'Hiragino Sans', 'Malgun Gothic', sans-serif";
  const WIDE = "Verdana, 'Microsoft YaHei', 'Malgun Gothic', 'Yu Gothic', sans-serif";
  const MONO = "ui-monospace, 'Cascadia Code', 'SF Mono', Consolas, monospace";
  const SIZE = { head: 22, title: 24, note: 18, code: 17, cap: 16, num: 17 };
  const TITLE_LH = 30, NOTE_LH = 25, ROW_PAD = 13, CHIP_H = 32, SAFE = 1.03;
  const THEMES = {
    light: { bg: "#F5F8FE", border: "#DCE5F4", card: "#FFFFFF", card2: "#F3F5F9", cardBorder: "#D3DDEE", dash: "#B6C1D2", chip: "#EAF2FF", ink: "#0F1B33", muted: "#53637D", blue: "#0F56D9", warn: "#C2410C", warnChip: "#FDEEE6", badge: "#0F56D9", badgeInk: "#FFFFFF", shadow: "#0F56D9", shadowOpacity: 0.1 },
    dark: { bg: "#0B1120", border: "#24324D", card: "#121C31", card2: "#16213A", cardBorder: "#2C3B58", dash: "#4A5B7C", chip: "#1B3155", ink: "#E8EEF9", muted: "#A1B1CC", blue: "#8DBDFF", warn: "#FFA27E", warnChip: "#3A2419", badge: "#8DBDFF", badgeInk: "#0B1120", shadow: "#000000", shadowOpacity: 0.35 },
  };

  const ctx = document.createElement("canvas").getContext("2d");
  const rawWidth = (s, size, weight, family) => {
    ctx.font = `${weight} ${size}px ${family}`;
    return ctx.measureText(s).width;
  };
  const fit = (s, size, weight = 400) => Math.max(rawWidth(s, size, weight, SANS), rawWidth(s, size, weight, WIDE)) * SAFE;
  // Monospace fallbacks (Menlo, DejaVu Sans Mono, Courier) advance 0.6em; Consolas is narrower.
  const chipWidth = (s) => Math.ceil(s.length * SIZE.code * 0.6 + 28);
  const esc = (s) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  const n = (v) => +v.toFixed(1);

  // --- line breaking ------------------------------------------------------
  const NO_START = new Set(Array.from(",.!?;:)]}，。！？、；：）」』》”’…"));
  const NO_END = new Set(Array.from("([{（「『《“‘"));
  function tokens(text, lang) {
    const out = [];
    let lead = "", pending = null;
    for (const { segment } of new Intl.Segmenter(lang, { granularity: "word" }).segment(text)) {
      if (/^\s+$/.test(segment)) { lead += segment; continue; }
      const chars = Array.from(segment);
      if (out.length && !pending && chars.every((c) => NO_START.has(c))) { out[out.length - 1].s += lead + segment; lead = ""; continue; }
      if (chars.every((c) => NO_END.has(c))) { pending = { s: (pending ? pending.s : "") + segment, lead: pending ? pending.lead : lead }; lead = ""; continue; }
      out.push({ s: (pending ? pending.s : "") + segment, lead: pending ? pending.lead : lead });
      pending = null; lead = "";
    }
    if (pending) out.push(pending);
    return out;
  }
  const lineText = (toks) => toks.map((t, i) => (i ? t.lead : "") + t.s).join("");
  function greedy(toks, limit, size, weight) {
    const rows = [];
    let cur = [];
    for (const t of toks) {
      if (cur.length && fit(lineText([...cur, t]), size, weight) > limit) { rows.push(cur); cur = []; }
      cur.push(t);
    }
    if (cur.length) rows.push(cur);
    return rows;
  }
  // Balanced: the narrowest width that keeps the greedy line count, so the last line is never a stub.
  function wrap(text, limit, size, weight, lang) {
    const toks = tokens(text, lang);
    let rows = greedy(toks, limit, size, weight);
    if (rows.length > 1) {
      let lo = Math.max(...toks.map((t) => fit(t.s, size, weight))), hi = limit;
      for (let i = 0; i < 16; i++) {
        const mid = (lo + hi) / 2;
        if (greedy(toks, mid, size, weight).length === rows.length) hi = mid; else lo = mid;
      }
      rows = greedy(toks, hi, size, weight);
    }
    return rows.map(lineText);
  }

  // --- drawing ------------------------------------------------------------
  const txt = (x, y, s, o) => {
    const a = [`x="${n(x)}"`, `y="${n(y)}"`, `font-family="${o.mono ? MONO : SANS}"`, `font-size="${o.size}"`, `fill="${o.fill}"`];
    if (o.weight) a.push(`font-weight="${o.weight}"`);
    if (o.anchor) a.push(`text-anchor="${o.anchor}"`);
    if (o.t) a.push(`data-t="${o.t}"`);
    if (o.k) a.push(`data-k="${o.k}" data-g="${o.k}"`);
    return `<text ${a.join(" ")}>${esc(s)}</text>`;
  };
  const textLines = (x, y, arr, pitch, o) => arr.map((s, i) => txt(x, y + i * pitch, s, o)).join("");

  function build(copy, th, lang, diagram) {
    const body = [];
    const star = (x, y, id) => `<path id="${id}" data-role="icon" d="M17.6 9.4c.3 5.3 1.2 6.5 5.2 7-4 .6-4.9 2-5.2 7.1-.6-5.1-1.3-6.4-5.5-7.1 4.2-.6 4.9-1.8 5.5-7Z" fill="url(#spark)" transform="translate(${n(x)} ${n(y)}) scale(1.7) translate(-17.5 -16.45)"/>`;
    const rect = (role, x, y, w, h, rx, extra) => `<rect data-role="${role}" x="${n(x)}" y="${n(y)}" width="${n(w)}" height="${n(h)}" rx="${rx}" ${extra}/>`;
    const card = (x, y, w, h, o = {}) => {
      const stroke = o.accent ? 'stroke="url(#spark)" stroke-width="2"' : o.dashed ? `stroke="${th.dash}" stroke-width="2" stroke-dasharray="8 6"` : `stroke="${th.cardBorder}" stroke-width="1.5"`;
      return rect("card", x, y, w, h, 14, `fill="${o.dashed ? th.card2 : th.card}" ${stroke}${o.dashed ? "" : ' filter="url(#shadow)"'}`);
    };
    const chip = (x, cy, s, o = {}) => {
      const w = o.width || chipWidth(s);
      return rect("chip", x, cy - CHIP_H / 2, w, CHIP_H, 8, `fill="${o.bg || th.chip}" data-in="${o.gap ? "gap" : "card"}"`) + txt(x + w / 2, cy + 6, s, { size: SIZE.code, fill: o.fill || th.blue, mono: 1, anchor: "middle", t: "code" });
    };
    const arrowHead = (x, y) => `<path data-role="arrow" d="M${n(x - 7)} ${n(y - 11)}L${n(x)} ${n(y)}L${n(x + 7)} ${n(y - 11)}Z" fill="${th.blue}" stroke="${th.blue}" stroke-width="1.5" stroke-linejoin="round"/>`;
    const arrowLine = (d, dotted) => `<path data-role="arrow" d="${d}" fill="none" stroke="${th.blue}" stroke-width="${dotted ? 3 : 2.5}" stroke-linecap="round"${dotted ? ' stroke-dasharray="1 8"' : ""}/>`;
    const down = (x, y1, y2) => arrowLine(`M${n(x)} ${n(y1 + 3)}V${n(y2 - 10)}`) + arrowHead(x, y2 - 1);
    const pill = (x, cy, s, key) => {
      const w = Math.ceil(fit(s, SIZE.cap, 600) + 28);
      return rect("pill", x, cy - 13, w, 26, 13, `fill="${th.chip}"`) + txt(x + w / 2, cy + 5.5, s, { size: SIZE.cap, weight: 600, fill: th.blue, anchor: "middle", t: "caption", k: key });
    };

    const textX = M + PAD + BADGE + 16, right = M + CW - PAD;
    const titleWidth = (spec) => fit(spec.title, SIZE.title, 700);
    const STAR_GAP = 30; // room the sparkle takes beside a command chip
    const roomForNote = (spec) => right - textX - chipWidth(spec.chip) - 28 - (spec.star ? STAR_GAP : 0);

    // One wide step: badge, title, note, command on the right.
    function wideRow(spec, stacked, column) {
      const tx = textX, cw = chipWidth(spec.chip);
      const avail = roomForNote(spec);
      const titleLines = wrap(spec.title, avail, SIZE.title, 700, lang);
      const noteLines = stacked ? wrap(spec.note, avail, SIZE.note, 400, lang) : [spec.note];
      const block = TITLE_LH * titleLines.length + 2 + NOTE_LH * noteLines.length;
      const h = stacked ? 2 * ROW_PAD + block : 60;
      return {
        h,
        draw(y) {
          const cy = y + h / 2, out = [card(M, y, CW, h, spec.cardOpts)];
          out.push(`<circle data-role="badge" cx="${CX}" cy="${n(cy)}" r="${BADGE / 2}" fill="${spec.badge}"/>`);
          out.push(txt(CX, cy + 6, String(spec.num), { size: SIZE.num, weight: 700, fill: th.badgeInk, anchor: "middle", t: "number" }));
          if (stacked) {
            const top = y + (h - block) / 2;
            out.push(textLines(tx, top + 23, titleLines, TITLE_LH, { size: SIZE.title, weight: 700, fill: th.ink, t: "title", k: spec.key }));
            out.push(textLines(tx, top + TITLE_LH * titleLines.length + 20, noteLines, NOTE_LH, { size: SIZE.note, fill: th.muted, t: "note", k: spec.noteKey }));
          } else {
            out.push(txt(tx, cy + 8.5, spec.title, { size: SIZE.title, weight: 700, fill: th.ink, t: "title", k: spec.key }));
            out.push(txt(textX + column + 22, cy + 8.5, spec.note, { size: SIZE.note, fill: th.muted, t: "note", k: spec.noteKey }));
          }
          if (spec.star) out.push(star(right - cw - 24, cy, spec.star));
          out.push(chip(right - cw, cy, spec.chip, spec.chipStyle));
          return out.join("");
        },
      };
    }

    // The gap between two cards: an arrow down the rail, plus whatever the arrow carries.
    function connector(opts) {
      const list = opts.list;
      const h = list ? 2 * 36 + 14 : opts.caption ? 36 : 30;
      return {
        h,
        draw(y) {
          const cy = y + h / 2, out = [down(CX, y, y + h)];
          if (opts.caption) out.push(pill(CX + 18, cy, opts.caption, opts.captionKey));
          if (list) {
            const x0 = CX + 26, labelW = fit(list.label, SIZE.note), chipW = Math.max(...list.rows.map((r) => chipWidth(r.chip)));
            const chipX = x0 + labelW + 18, noteX = chipX + chipW + 12;
            out.push(txt(x0, cy + 6, list.label, { size: SIZE.note, fill: th.muted, t: "note", k: list.labelKey }));
            list.rows.forEach((r, i) => {
              const ry = cy + (i ? 18 : -18);
              out.push(chip(chipX, ry, r.chip, { width: chipW, gap: 1 }));
              out.push(txt(noteX, ry + 6, r.note, { size: SIZE.note, fill: th.muted, t: "note", k: r.noteKey }));
            });
          }
          return out.join("");
        },
      };
    }

    // A card that sits beside its sibling under a fork.
    function halfCard(spec, w, inline) {
      const px = 20, cw = chipWidth(spec.chip);
      const noteLines = wrap(spec.note, w - 2 * px, SIZE.note, 400, lang);
      const head = inline ? CHIP_H + 4 : TITLE_LH + 6 + CHIP_H;
      const h = 14 + head + 8 + NOTE_LH * noteLines.length + 10;
      return {
        h,
        draw(x, y, hh) {
          const tx = x + px, out = [card(x, y, w, hh, spec.cardOpts)];
          const rowCy = y + 14 + (CHIP_H + 4) / 2;
          const titleY = inline ? rowCy + 8.5 : y + 14 + 23;
          out.push(txt(tx, titleY, spec.title, { size: SIZE.title, weight: 700, fill: th.ink, t: "title", k: spec.key }));
          const chipX = inline ? x + w - px - cw : x + px, chipCy = inline ? rowCy : y + 14 + TITLE_LH + 6 + CHIP_H / 2;
          if (spec.star) out.push(star(inline ? chipX - 24 : chipX + cw + 24, chipCy, spec.star));
          out.push(chip(chipX, chipCy, spec.chip));
          out.push(textLines(x + px, y + 14 + head + 8 + 17, noteLines, NOTE_LH, { size: SIZE.note, fill: th.muted, t: "note", k: spec.noteKey }));
          return out.join("");
        },
      };
    }

    let heading = "", y = 24, defsTitle, desc, rowsSpec, tail;
    if (diagram === "lifecycle") {
      heading = txt(M + 2, 44, copy.title, { size: SIZE.head, weight: 700, fill: th.muted, t: "heading", k: "title" });
      y = 62;
      defsTitle = copy.title; desc = copy.life_desc;
      rowsSpec = [
        { num: 1, title: copy.register, key: "register", note: copy.register_note, noteKey: "register_note", chip: "obit add debug.log 2h", badge: th.badge },
        { num: 2, title: copy.expire, key: "expire", note: copy.expire_note, noteKey: "expire_note", chip: "obit list", badge: th.badge },
        { num: 3, title: copy.bury, key: "bury", note: copy.bury_note, noteKey: "bury_note", chip: "obit graveyard", badge: th.badge },
      ];
      tail = [
        connector({ list: { label: copy.before, labelKey: "before", rows: [{ chip: "obit extend", note: copy.extend_note, noteKey: "extend_note" }, { chip: "obit spare", note: copy.spare_note, noteKey: "spare_note" }] } }),
        connector({ caption: copy.reap, captionKey: "reap" }),
      ];
    } else {
      defsTitle = `${copy.guard_title}: Obituary`; desc = copy.guard_desc;
      rowsSpec = [
        { num: 1, title: copy.guard_title, key: "guard_title", note: copy.guard_first, noteKey: "guard_first", chip: "rm -rf build", badge: th.warn, chipStyle: { bg: th.warnChip, fill: th.warn } },
        { num: 2, title: copy.guard_equiv, key: "guard_equiv", note: copy.guard_second, noteKey: "guard_second", chip: "obit rm -rf build", badge: th.badge },
        { num: 3, title: copy.guard_res, key: "guard_res", note: copy.guard_res_note, noteKey: "guard_res_note", chip: "obit resurrect <id>", badge: th.badge, star: "guard-star", cardOpts: { accent: 1 } },
      ];
      tail = [connector({ caption: copy.guard_bridge, captionKey: "guard_bridge" }), connector({})];
    }
    // The note sits beside its title only when every row has room for that; otherwise every row stacks.
    const column = Math.max(...rowsSpec.map(titleWidth));
    const stacked = !rowsSpec.every((s) => column + 22 + fit(s.note, SIZE.note) <= roomForNote(s));
    rowsSpec.map((s) => wideRow(s, stacked, column)).forEach((r, i) => {
      body.push(r.draw(y)); y += r.h;
      if (tail[i]) { body.push(tail[i].draw(y)); y += tail[i].h; }
    });

    if (diagram === "lifecycle") {
      const gap = 16, halfW = (CW - gap) / 2;
      const specA = { title: copy.resurrect, key: "resurrect", note: copy.resurrect_note, noteKey: "resurrect_note", chip: "obit resurrect 3", star: "resurrect-star", cardOpts: { accent: 1 } };
      const specB = { title: copy.purge, key: "purge", note: copy.purge_note, noteKey: "purge_note", chip: "obit purge", cardOpts: { dashed: 1 } };
      const sideBySide = [specA, specB].every((c) => titleWidth(c) + 20 + chipWidth(c.chip) + (c.star ? STAR_GAP : 0) <= halfW - 40);
      const a = halfCard(specA, halfW, sideBySide), b = halfCard(specB, halfW, sideBySide);
      const h = Math.max(a.h, b.h), bx = M + halfW + gap, bcx = bx + halfW / 2;
      const forkH = 30, midY = y + forkH / 2;
      body.push(down(CX, y, y + forkH));
      body.push(arrowLine(`M${CX} ${n(midY)}H${n(bcx - 14)}Q${n(bcx)} ${n(midY)} ${n(bcx)} ${n(midY + 14)}V${n(y + forkH - 10)}`, true) + arrowHead(bcx, y + forkH - 1));
      y += forkH;
      body.push(a.draw(M, y, h), b.draw(bx, y, h));
      y += h;
    }

    const H = Math.ceil(y + 22);
    const head = [
      `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}" role="img" aria-labelledby="title desc">`,
      `<title id="title">${esc(defsTitle)}</title><desc id="desc">${esc(desc)}</desc>`,
      `<defs><linearGradient id="spark" x1="0" y1="1" x2="1" y2="0"><stop stop-color="#0876FF"/><stop offset=".55" stop-color="#16B8FA"/><stop offset=".62" stop-color="#FFE052"/><stop offset="1" stop-color="#FFB329"/></linearGradient>`,
      `<filter id="shadow" x="-4%" y="-12%" width="108%" height="132%"><feDropShadow dx="0" dy="2" stdDeviation="3" flood-color="${th.shadow}" flood-opacity="${th.shadowOpacity}"/></filter></defs>`,
      `<rect data-role="bg" x="1" y="1" width="${W - 2}" height="${H - 2}" rx="24" fill="${th.bg}" stroke="${th.border}" stroke-width="2"/>`,
      heading,
    ];
    return head.join("\n") + "\n" + body.join("\n") + "\n</svg>\n";
  }

  window.buildAll = (strings) => {
    const out = {};
    for (const [lang, copy] of Object.entries(strings)) {
      for (const diagram of ["lifecycle", "delete-guard"]) {
        for (const [theme, th] of Object.entries(THEMES)) out[`${lang}/${diagram}-${theme}.svg`] = build(copy, th, lang, diagram);
      }
    }
    return out;
  };
})();
