import type { ReactNode } from "react";

import { localeOf, type Lang } from "@/locales";
import { cn } from "@/lib/utils";

// The flags of the language menu, drawn in code so they come with the page itself: no file of their
// own to request, nothing that can arrive broken. Each flag has its legal proportions (width:height)
// and is defined once per page in a hidden sprite (<FlagSprite/>), then shown by reference wherever
// it is used. Sources and licences: site/THIRD_PARTY_NOTICES.md.
type Flag = { w: number; h: number; art: ReactNode };

const bars = (w: number, fills: string[], horizontal = true) =>
  fills.map((fill, i) =>
    horizontal ? <rect key={i} y={(i * 600) / fills.length} width={w} height={600 / fills.length} fill={fill} /> : <rect key={i} x={(i * w) / fills.length} width={w / fills.length} height="600" fill={fill} />,
  );

// A five-pointed star, points up, outer radius r, at (x, y).
const star = (x: number, y: number, r: number) => {
  const pts = Array.from({ length: 10 }, (_, i) => {
    const a = ((i * 36 - 90) * Math.PI) / 180;
    const d = i % 2 ? r * 0.381966 : r;
    return `${(x + d * Math.cos(a)).toFixed(1)},${(y + d * Math.sin(a)).toFixed(1)}`;
  });
  return pts.join(" ");
};

// The 50 stars of the United States: 11 columns 247 apart, 5 stars in the odd ones and 4 in the even.
const usStars = Array.from({ length: 11 }, (_, k) =>
  Array.from({ length: k % 2 ? 4 : 5 }, (_, j) => [247 + 247 * k, (k % 2 ? 420 : 210) + 420 * j] as const),
).flat();

// The 27 stars of Brazil's flag: [x, y, outer radius] in the flag's own units.
const brStars: [number, number, number][] = [
  [-600, -132, 31.5], [-535, 177, 31.5], [-625, 243, 26.25], [-463, 132, 15], [-382, 250, 26.25], [-404, 323, 21],
  [228, -228, 31.5], [515, 258, 31.5], [617, 265, 21], [545, 323, 26.25], [368, 477, 26.25], [367, 551, 21], [441, 419, 21],
  [500, 382, 26.25], [365, 405, 21], [-280, 30, 26.25], [200, -37, 21], [0, 330, 31.5], [85, 184, 26.25], [0, 118, 26.25],
  [-74, 184, 21], [-37, 235, 15], [220, 495, 26.25], [283, 430, 21], [162, 412, 21], [-295, 390, 31.5], [0, 575, 10.5],
];

const flags: Record<string, Flag & { box?: string }> = {
  // 3:2
  cn: {
    w: 3, h: 2,
    art: (
      <>
        <path fill="#EE1C25" d="M0 0h900v600H0" />
        <path id="fl-cn-star" d="m0-30 17.634 54.27-46.166-33.54h57.064l-46.166 33.54Z" fill="#FF0" />
        <use href="#fl-cn-star" transform="matrix(3 0 0 3 150 150)" />
        <use href="#fl-cn-star" transform="rotate(23.036 2.784 766.082)" />
        <use href="#fl-cn-star" transform="rotate(45.87 38.201 485.396)" />
        <use href="#fl-cn-star" transform="rotate(69.945 29.892 362.328)" />
        <use href="#fl-cn-star" transform="rotate(20.66 -590.66 957.955)" />
      </>
    ),
  },
  jp: {
    w: 3, h: 2,
    art: (
      <>
        <rect width="900" height="600" fill="#fff" />
        <circle cx="450" cy="300" r="180" fill="#bc002d" />
      </>
    ),
  },
  kr: {
    w: 3, h: 2, box: "-72 -48 144 96",
    art: (
      <>
        <path fill="#fff" d="M-72-48v96H72v-96z" />
        <g stroke="#000" strokeWidth="4">
          <path transform="rotate(33.69006752598)" d="M-50-12v24m6 0v-24m6 0v24m76 0V1m0-2v-11m6 0v11m0 2v11m6 0V1m0-2v-11" />
          <path transform="rotate(-33.69006752598)" d="M-50-12v24m6 0V1m0-2v-11m6 0v24m76 0V1m0-2v-11m6 0v24m6 0V1m0-2v-11" />
        </g>
        <g transform="rotate(33.69006752598)">
          <path fill="#cd2e3a" d="M12 0a18 18 0 11-36 0 24 24 0 1148 0" />
          <path fill="#0047a0" d="M-24 0a24 24 0 1048 0A12 12 0 100 0a12 12 0 11-24 0" />
        </g>
      </>
    ),
  },
  es: {
    w: 3, h: 2,
    art: (
      <>
        <rect width="900" height="600" fill="#aa151b" />
        <rect y="150" width="900" height="300" fill="#f1bf00" />
      </>
    ),
  },
  fr: { w: 3, h: 2, art: bars(900, ["#002395", "#fff", "#ed2939"], false) },
  ru: { w: 3, h: 2, art: bars(900, ["#fff", "#0039a6", "#d52b1e"]) },
  // 5:3
  de: { w: 5, h: 3, art: bars(1000, ["#000", "#d00", "#ffce00"]) },
  // 19:10
  us: {
    w: 19, h: 10, box: "0 0 7410 3900",
    art: (
      <>
        <path d="M0,0h7410v3900H0" fill="#b31942" />
        <path d="M0,450H7410m0,600H0m0,600H7410m0,600H0m0,600H7410m0,600H0" stroke="#FFF" strokeWidth="300" />
        <path d="M0,0h2964v2100H0" fill="#0a3161" />
        {usStars.map(([x, y], i) => <polygon key={i} points={star(x, y, 120)} fill="#fff" />)}
      </>
    ),
  },
  // 10:7
  br: {
    w: 10, h: 7, box: "-2100 -1470 4200 2940",
    art: (
      <>
        <clipPath id="fl-br-disc"><circle r="735" /></clipPath>
        <path d="m-2100-1470h4200v2940h-4200z" fill="#009440" />
        <path d="M -1743,0 0,1113 1743,0 0,-1113 Z" fill="#ffcb00" />
        <circle r="735" fill="#302681" />
        <path d="m-2205 1470a1785 1785 0 0 1 3570 0h-105a1680 1680 0 1 0-3360 0z" clipPath="url(#fl-br-disc)" fill="#fff" />
        {brStars.map(([x, y, r], i) => <polygon key={i} points={star(x, y, r)} fill="#fff" />)}
      </>
    ),
  },
};

const viewBox = (f: Flag & { box?: string }) => f.box ?? `0 0 ${f.w === 3 ? 900 : f.w === 5 ? 1000 : 1000} 600`;

// One definition of each flag per page, kept out of sight; every <Flag/> points at it.
export function FlagSprite() {
  return (
    <svg aria-hidden="true" focusable="false" width="0" height="0" className="pointer-events-none absolute size-0 overflow-hidden">
      {Object.entries(flags).map(([id, f]) => (
        <symbol key={id} id={`flag-${id}`} viewBox={viewBox(f)}>{f.art}</symbol>
      ))}
    </svg>
  );
}

const HEIGHT = 16;

// All flags are 16px tall and as wide as their proportions say, centred in a slot of one width so the
// names beside them line up.
export function Flag({ id, className }: { id: Lang; className?: string }) {
  const key = localeOf(id).flag;
  const f = flags[key];
  const width = Math.round(((HEIGHT * f.w) / f.h) * 100) / 100;
  return (
    <span aria-hidden="true" className={cn("grid h-4 w-8 shrink-0 place-items-center", className)}>
      <svg width={width} height={HEIGHT} viewBox={`0 0 ${f.w} ${f.h}`} focusable="false" data-flag={key} className="overflow-hidden rounded-[2px] ring-1 ring-ink/15">
        <use href={`#flag-${key}`} width={f.w} height={f.h} />
      </svg>
    </span>
  );
}
