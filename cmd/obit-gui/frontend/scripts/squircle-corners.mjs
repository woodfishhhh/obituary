// Build half of the squircle corners; the run-time half is src/squircle.css.
//
// Every border-radius that draws a rounded rectangle is rewritten to
//   border-radius: calc(<radius> * var(--squircle-k, 1));
//   corner-shape: var(--squircle-shape, round);
// Engines that cannot draw `corner-shape: squircle` never define the two variables, so they compute the original
// radius and the original circular corner; the others get them from src/squircle.css. Radii are not scaled
// by hand anywhere: write a plain border-radius, the build does the rest.
//
// Left alone, with an explicit round corner: percentages (circles), radii of 100px and up and the Tailwind
// `calc(infinity * 1px)` (pills), because a pill must keep circular ends. Left alone, nothing added: zero and
// radii under 5px, where the two shapes cannot be told apart. `border-radius: inherit` also inherits the shape.
//
// Written here instead of taking postcss-smooth-corners (postcss org, `auto` mode): its ratio is a constant
// baked into every rule, 1.715, which is not the fit to Figma's 60% smoothing (src/squircle.css), and it does not
// scale radii written as var() or calc(), which is how every radius token of the window and of Tailwind is
// written. Concentric corners the author composes by hand (`calc(var(--r) * var(--squircle-k, 1) + 3px)`) are
// taken as they are. A rule that sets `corner-shape` itself is left entirely alone.
//
// No imports: the landing page and the docs build from site/ alone, without this package's node_modules.

const RADIUS = /^border(-(top|bottom)-(left|right)|-(start|end)-(start|end))?-radius$/;
const ZERO = /^0+(\.0+)?([a-z%]*)$/;
const LENGTH = /^(\d*\.?\d+)(px|rem|em)$/;
const SCALED = "--squircle-k";
const SHAPE = "var(--squircle-shape, round)";

// The words of a radius value, split at the top level only: `calc(1px + 2px)` and `var(--a, 3px)` stay whole.
function words(value) {
  const out = [];
  let depth = 0;
  let word = "";
  const flush = () => { if (word) out.push(word); word = ""; };
  for (const ch of value.trim()) {
    if (ch === "(") depth++;
    if (ch === ")") depth--;
    if (depth === 0 && (/\s/.test(ch) || ch === "/")) {
      flush();
      if (ch === "/") out.push("/");
    } else {
      word += ch;
    }
  }
  flush();
  return out;
}

// What a word of a radius value asks for: "scale" (a rounded rectangle), "done" (scaled by hand), "round" (a
// circle or a pill), "inherit", or "keep" (zero, tiny, a slash, a CSS-wide keyword).
function kind(word) {
  if (word === "inherit") return "inherit";
  if (word === "/" || ZERO.test(word) || /^(initial|unset|revert|revert-layer)$/.test(word)) return "keep";
  if (word.endsWith("%") || word.includes("infinity")) return "round";
  if (word.includes(SCALED)) return "done";
  const length = LENGTH.exec(word);
  if (!length) return "scale"; // var(), calc(), env(): the radius tokens
  const px = parseFloat(length[1]) * (length[2] === "px" ? 1 : 16);
  return px >= 100 ? "round" : px < 5 ? "keep" : "scale";
}

const scaled = (word) => {
  const inner = /^calc\((.*)\)$/s.exec(word)?.[1];
  return `calc(${inner === undefined ? word : `(${inner})`} * var(${SCALED}, 1))`;
};

// Rewrites one declaration and says which corner shape it needs: "squircle", "round", "inherit" or "".
function rewrite(decl, hold) {
  const parts = words(decl.value);
  const kinds = parts.map(kind);
  if (hold) return "round";
  if (kinds.includes("inherit")) return "inherit";
  if (kinds.includes("scale") || kinds.includes("done")) {
    decl.value = parts.map((word, i) => (kinds[i] === "scale" ? scaled(word) : word)).join(" ");
    return "squircle";
  }
  return kinds.includes("round") ? "round" : "";
}

const inKeyframes = (rule) => rule.parent?.type === "atrule" && /keyframes$/i.test(rule.parent.name);

/** @param {{ roundSelectors?: RegExp }} [options] roundSelectors: rules whose corners stay circular (pills written with a small px radius). */
export default function squircleCorners(options = {}) {
  return {
    postcssPlugin: "obit-squircle-corners",
    Once(root) {
      root.walkRules((rule) => {
        if (inKeyframes(rule) || rule.nodes.some((n) => n.type === "decl" && n.prop === "corner-shape")) return;
        const radii = rule.nodes.filter((n) => n.type === "decl" && RADIUS.test(n.prop));
        if (radii.length === 0) return;
        const hold = options.roundSelectors?.test(rule.selector) ?? false;
        const shapes = radii.map((decl) => rewrite(decl, hold));
        const shape = ["squircle", "inherit", "round"].find((s) => shapes.includes(s));
        if (shape) radii[radii.length - 1].cloneAfter({ prop: "corner-shape", value: shape === "squircle" ? SHAPE : shape });
      });
    },
  };
}
squircleCorners.postcss = true;
