// 外观: the shipped looks, each as a small drawing of its window. A press
// switches at once; the choice is kept on this computer only. The language
// sits beside it in Settings, kept by the backend.
import { useId } from "react";
import { useAppearance } from "../appearance";
import { t } from "../i18n";

export default function Appearance() {
  const a = useAppearance();
  const title = useId();
  if (!a) return null;
  return (
    <section className="looks" aria-labelledby={title}>
      <div className="section-title" id={title}>{t("look.title")}</div>
      <p className="hint looks-hint">{t("look.hint")}</p>
      <div className="looks-grid">
        {a.looks.map((l) => (
          <button key={l.id} type="button" className="look" aria-pressed={a.current === l.id} aria-busy={a.loading === l.id} onClick={() => a.choose(l.id)}>
            <Thumb id={l.id} />
            <b>{l.name}{a.current === l.id ? <small>{t("look.current")}</small> : null}</b>
            <span>{l.note}</span>
          </button>
        ))}
      </div>
    </section>
  );
}

// The colours here are each look's own, drawn as a picture of it.
// A gate of the tunnel: the logo's tombstone outline, open at the bottom.
const gate = (x: number, y: number, w: number, h: number) =>
  `M${x} ${y + h}V${y + h * 0.18}L${x + w * 0.14} ${y}H${x + w * 0.84}L${x + w} ${y + h * 0.18}V${y + h}`;
const gates: [number, number, number, number, string][] = [[6, 6, 84, 54, "#F8E000"], [20, 15, 56, 38, "#E2EEFF"], [30, 21, 36, 25, "#5FA6FF"]];

function Thumb({ id }: { id: string }) {
  const u = useId().replace(/:/g, "");
  const band = `${u}b`;
  const glow = `${u}g`;
  const common = (
    <defs>
      <linearGradient id={band} x1="0" x2="1">
        <stop offset="0" stopColor="#0048E8" /><stop offset="0.35" stopColor="#0078F8" /><stop offset="0.62" stopColor="#B9D9FF" /><stop offset="0.8" stopColor="#FCEF8A" /><stop offset="1" stopColor="#F8E000" />
      </linearGradient>
      <radialGradient id={glow}>
        <stop offset="0" stopColor={id === "vigil" ? "#F8E000" : "#C4E0FF"} stopOpacity={id === "vigil" ? 0.55 : 0.9} /><stop offset="1" stopColor={id === "vigil" ? "#F8E000" : "#0A3CC8"} stopOpacity="0" />
      </radialGradient>
    </defs>
  );
  const body = (() => {
    switch (id) {
      case "calendar":
        return (<>
          <rect width="96" height="60" fill="#FFFFFF" />
          <rect width="96" height="8" fill="#F3F5FA" />
          <rect x="4" y="15" width="88" height="2" fill={`url(#${band})`} />
          {[0, 1, 2, 3, 4, 5].map((i) => <rect key={i} x={4 + i * 14.7} y="10" width="0.6" height="48" fill="#E6E9F0" />)}
          <rect x="6" y="20" width="11" height="8" rx="1.5" fill="#0048E8" />
          <rect x="6" y="30" width="11" height="8" rx="1.5" fill="#FFFFFF" stroke="#D0D6E2" strokeWidth="0.6" />
          <rect x="35.5" y="20" width="11" height="8" rx="1.5" fill="#FFFFFF" stroke="#D0D6E2" strokeWidth="0.6" />
          <rect x="65" y="20" width="11" height="8" rx="1.5" fill="#FFFFFF" stroke="#D0D6E2" strokeWidth="0.6" />
        </>);
      case "palette":
        return (<>
          <rect width="96" height="60" fill="#E6EBF5" />
          <circle cx="4" cy="0" r="40" fill="#0048E8" opacity="0.35" />
          <circle cx="96" cy="60" r="36" fill="#F8E000" opacity="0.5" />
          <rect x="6" y="5" width="84" height="50" rx="4" fill="#FAFBFE" opacity="0.88" />
          <rect x="11" y="10" width="50" height="4" rx="2" fill="#10162A" opacity="0.55" />
          <rect x="6" y="18" width="84" height="0.6" fill="#10162A" opacity="0.12" />
          <rect x="9" y="22" width="78" height="7" rx="2" fill="#0048E8" opacity="0.1" />
          <rect x="9" y="23.5" width="1.2" height="4" fill="#0048E8" />
          {[22, 31, 40].map((y) => <rect key={y} x="14" y={y + 2.5} width={y === 22 ? 36 : 28} height="2" rx="1" fill="#3A4359" opacity="0.6" />)}
        </>);
      case "tunnel":
        return (<>
          <rect width="96" height="60" fill="#061F78" />
          <ellipse cx="48" cy="32" rx="40" ry="26" fill={`url(#${glow})`} />
          {gates.map(([x, y, w, h, c]) => <path key={c} d={gate(x, y, w, h)} fill="none" stroke={c} strokeWidth="1.2" />)}
          <rect x="22" y="30" width="12" height="9" rx="1" fill="#FFFFFF" />
          <rect x="58" y="24" width="8" height="6" rx="1" fill="#FFFFFF" opacity="0.9" />
        </>);
      case "vigil":
        return (<>
          <rect width="96" height="60" fill="#070A14" />
          <rect x="34" y="0" width="0.6" height="60" fill="#1A2236" />
          {[14, 26, 38, 50].map((y, i) => <g key={y}><circle cx="9" cy={y} r={5 - i} fill={`url(#${glow})`} /><circle cx="9" cy={y} r="1.3" fill="#F8E000" opacity={1 - i * 0.2} /><rect x="16" y={y - 1} width="14" height="2" rx="1" fill="#B4BBCC" opacity="0.5" /></g>)}
          <circle cx="58" cy="22" r="22" fill={`url(#${glow})`} />
          <text x="46" y="36" fontFamily="Georgia, serif" fontSize="26" fontWeight="300" fill="#ECEEF5">6</text>
          <rect x="46" y="44" width="36" height="1.6" rx="0.8" fill="#B4BBCC" opacity="0.5" />
        </>);
      case "notice":
        return (<>
          <rect width="96" height="60" fill="#F4F2EA" />
          <path d="M36 10C44 6 54 4 62 4" stroke="#F8E000" strokeWidth="1.4" fill="none" />
          <rect x="40" y="4" width="16" height="6" fill="#0048E8" />
          <rect x="4" y="13" width="88" height="1.4" fill="#16181D" />
          <rect x="4" y="15.4" width="88" height="0.5" fill="#16181D" />
          <rect x="6" y="20" width="44" height="3" fill="#16181D" />
          {[0, 1, 2].map((c) => [0, 1, 2, 3, 4].map((r) => <rect key={`${c}${r}`} x={6 + c * 30} y={28 + r * 5.5} width={r === 0 ? 18 : 24} height={r === 0 ? 2.2 : 1.2} fill="#3E4048" opacity={r === 0 ? 0.9 : 0.45} />))}
          <rect x="34" y="27" width="0.5" height="30" fill="#B9B5A8" /><rect x="64" y="27" width="0.5" height="30" fill="#B9B5A8" />
        </>);
      case "horizon":
        return (<>
          <rect width="96" height="34" fill="#1F6BE0" /><rect width="96" height="16" fill="#0B1D57" opacity="0.7" />
          <rect y="34" width="96" height="26" fill="#060D24" />
          <rect y="33.4" width="96" height="1.2" fill={`url(#${band})`} />
          {[[20, 12], [44, 22], [70, 16], [56, 28]].map(([x, y]) => <circle key={`${x}`} cx={x} cy={y} r="1.6" fill="#FFFFFF" />)}
          {[[26, 42], [60, 48], [42, 54]].map(([x, y]) => <g key={`${x}`}><rect x={x - 2} y={y} width="4" height="4" fill="#9DB8F5" /><path d={`M${x} ${y - 4}L${x + 0.7} ${y - 2.6}L${x + 2} ${y - 2}L${x + 0.7} ${y - 1.4}L${x} ${y}L${x - 0.7} ${y - 1.4}L${x - 2} ${y - 2}L${x - 0.7} ${y - 2.6}Z`} fill="#F8E000" /></g>)}
        </>);
      case "mica":
        return (<>
          <rect width="96" height="60" fill="#F3F3F3" />
          <rect x="4" y="8" width="18" height="5" rx="1.5" fill="#EAEAEA" /><rect x="4" y="9" width="1.2" height="3" fill="#0048E8" />
          {[16, 22, 28].map((y) => <rect key={y} x="6" y={y} width="12" height="1.6" rx="0.8" fill="#5C5C5C" opacity="0.5" />)}
          <rect x="27" y="6" width="64" height="48" rx="3" fill="#FBFBFB" stroke="#E5E5E5" strokeWidth="0.6" />
          {[14, 24, 34, 44].map((y) => <g key={y}><rect x="31" y={y} width="5" height="5" rx="1" fill="#F5F5F5" stroke="#E5E5E5" strokeWidth="0.5" /><rect x="39" y={y + 1.5} width="26" height="1.8" rx="0.9" fill="#1B1B1B" opacity="0.6" /><rect x="70" y={y + 2} width="16" height="1.2" rx="0.6" fill={`url(#${band})`} /></g>)}
        </>);
      default:
        return (<>
          <rect width="96" height="60" fill="#111214" />
          <rect width="96" height="10" fill="#111214" /><rect y="10" width="96" height="0.6" fill="#26272C" />
          <rect x="18" y="4" width="10" height="2" rx="1" fill="#EDEEF0" /><rect x="18" y="9" width="10" height="1" fill="#EDEEF0" />
          {[34, 46, 58].map((x) => <rect key={x} x={x} y="4" width="8" height="2" rx="1" fill="#8B8E96" />)}
          <rect x="6" y="16" width="84" height="38" rx="4" fill="#18191C" stroke="#26272C" strokeWidth="0.6" />
          {[22, 31, 40].map((y) => <g key={y}><rect x="11" y={y} width="30" height="2" rx="1" fill="#EDEEF0" opacity="0.7" /><rect x="58" y={y + 0.4} width="26" height="1.2" rx="0.6" fill="#3D8BFF" /></g>)}
          <circle cx="84" cy="49" r="1.6" fill="#F8E000" />
        </>);
    }
  })();
  return <svg className="look-thumb" viewBox="0 0 96 60" aria-hidden>{common}{body}</svg>;
}

