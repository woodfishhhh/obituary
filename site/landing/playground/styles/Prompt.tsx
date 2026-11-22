import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react";
import { agentById, agents, commandOf, docUrl, LogoTile, num, useCopied, usePg, words } from "../common";
import { logoColor, range, toggle, type Knob, type Values } from "../knobs";

export const knobs: Knob[] = [
  range("font", "字号", 12, 20, 1, 15, " px"),
  range("logoSize", "图标大小", 18, 40, 1, 26, " px"),
  range("typing", "打字速度（0 = 直接出现）", 0, 90, 5, 35, " ms/字"),
  toggle("names", "候选项显示名字", true),
  logoColor,
];

// The page's own command, as a prompt: type to narrow the list (or arrow into it), choose, and the command types itself.
export function Wall({ v }: { v: Values }) {
  const { selected, select, lang, reduce } = usePg();
  const t = words[lang];
  const [copied, copy] = useCopied();
  const [text, setText] = useState("");
  const chips = useRef<(HTMLButtonElement | null)[]>([]);
  const input = useRef<HTMLInputElement>(null);
  const a = agentById(selected);

  // Choosing types the agent's id after the prompt, one letter at a time.
  useEffect(() => {
    if (!a) return;
    const speed = reduce ? 0 : num(v.typing);
    if (speed === 0) { setText(a.id); return; }
    let i = 0;
    setText("");
    const timer = window.setInterval(() => { i += 1; setText(a.id.slice(0, i)); if (i >= a.id.length) window.clearInterval(timer); }, speed);
    return () => window.clearInterval(timer);
  }, [a?.id, v.typing, reduce]); // eslint-disable-line react-hooks/exhaustive-deps

  const q = (a ? "" : text).trim().toLowerCase();
  const shown = useMemo(() => (q ? agents.filter((x) => x.name.toLowerCase().includes(q) || x.id.includes(q)) : agents), [q]);

  const onInput = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter" && shown[0]) select(shown[0].id);
    if (e.key === "ArrowDown") { e.preventDefault(); chips.current[0]?.focus(); }
  };
  const onChip = (e: KeyboardEvent, i: number) => {
    const next = { ArrowRight: i + 1, ArrowLeft: i - 1 }[e.key];
    if (e.key === "ArrowUp") { e.preventDefault(); input.current?.focus(); }
    if (next === undefined) return;
    e.preventDefault();
    chips.current[Math.max(0, Math.min(shown.length - 1, next))]?.focus();
  };

  return (
    <div className="mx-auto max-w-[920px] px-5 @3xl:px-10">
      <div className="pg-term" style={{ fontSize: num(v.font) }}>
        <div className="pg-term-bar"><i /><i /><i /><span className="ml-2">~/my-project</span></div>
        <div className="grid gap-5 p-5 @md:p-6">
          <label className="pg-term-line">
            <span style={{ color: "#4f8bff" }}>❯</span>
            <span>obit init --agent</span>
            <input
              ref={input}
              value={text}
              placeholder={t.filter}
              spellCheck={false}
              autoComplete="off"
              aria-label={t.filter}
              onChange={(e) => { setText(e.target.value); if (selected) select(null); }}
              onKeyDown={onInput}
            />
          </label>
          <div role="group" aria-label={t.head} className="flex flex-wrap gap-2">
            {shown.map((x, i) => (
              <button
                key={x.id}
                ref={(el) => { chips.current[i] = el; }}
                type="button"
                className="pg-term-chip pg-item"
                data-on={selected === x.id || undefined}
                aria-pressed={selected === x.id}
                aria-label={v.names ? undefined : x.name}
                onClick={() => select(x.id)}
                onKeyDown={(e) => onChip(e, i)}
              >
                <LogoTile a={x} size={num(v.logoSize)} mode={String(v.logo)} />
                {v.names && <span>{x.name}</span>}
              </button>
            ))}
            {shown.length === 0 && <span style={{ color: "#8d97b8" }}>{t.none}</span>}
          </div>
          <div className="flex min-h-10 flex-wrap items-center gap-3 border-t border-[#263055] pt-4" aria-live="polite">
            {a ? (
              <>
                <code className="min-w-0 flex-1 overflow-x-auto whitespace-nowrap"><span style={{ color: "#4f8bff" }}>$ </span>{commandOf(a.id)}</code>
                <button type="button" className="pg-btn" onClick={() => copy(a.id, commandOf(a.id))}>{copied === a.id ? t.copied : t.copy}</button>
                <a className="pg-btn pg-btn-solid" href={docUrl(a.id, lang)} target="_blank" rel="noreferrer">{t.docs} ↗</a>
              </>
            ) : (
              <span style={{ color: "#8d97b8" }}>{t.pick}</span>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
