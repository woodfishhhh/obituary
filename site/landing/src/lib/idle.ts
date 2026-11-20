// The page's lower sections are real HTML from the start; their scripts wait until the visitor has a
// reason for them, so the first screen's hydration is not queued behind work nobody can see. This
// resolves on the first sign of the visitor (scroll, wheel, touch, key, pointer, focus, a #link), or a few
// seconds after the page has loaded, and at once on the server, where the build has nothing to wait for.
const SIGNS = ["scroll", "wheel", "touchstart", "keydown", "pointerdown", "pointermove", "focusin", "hashchange"] as const;
const AFTER_LOAD = 4500;

let gate: Promise<void> | undefined;
export function whenNeeded(): Promise<void> {
  if (typeof window === "undefined") return Promise.resolve();
  gate ??= new Promise<void>((resolve) => {
    let timer: ReturnType<typeof setTimeout> | undefined;
    const go = () => {
      clearTimeout(timer);
      SIGNS.forEach((s) => window.removeEventListener(s, go, true));
      resolve();
    };
    SIGNS.forEach((s) => window.addEventListener(s, go, { capture: true, passive: true, once: true }));
    const afterLoad = () => { timer = setTimeout(() => ("requestIdleCallback" in window ? requestIdleCallback(go, { timeout: 2000 }) : go()), location.hash ? 0 : AFTER_LOAD); };
    if (document.readyState === "complete") afterLoad();
    else window.addEventListener("load", afterLoad, { once: true });
  });
  return gate;
}
