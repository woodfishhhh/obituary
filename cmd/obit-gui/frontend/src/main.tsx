import { StrictMode, useCallback, useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import { ApiContext, defaultApi, useApi } from "./api";
import { AppearanceContext, savedLook, saveLook } from "./appearance";
import { looks } from "./catalog";
import { cachedLang, cacheLang, fromTag, LangContext, setLang, systemLang, t, type Lang } from "./i18n";
import "./styles.css";
import type { Variant } from "./theme";

// The look the app starts with until the user picks another under 设置 > 外观.
const SHIPPED = "calendar";

function Root() {
  const api = useApi();
  const [variant, setVariant] = useState<Variant | null>(null);
  const [loading, setLoading] = useState<string>();
  // The backend owns the language (config.toml, shared with the tray and the
  // context menu). Until it answers, nothing is drawn; an older backend
  // without the field, or none at all, falls back to Windows' language.
  const [current, setCurrent] = useState<Lang | null>(null);
  useEffect(() => {
    let live = true;
    api.Settings().then(
      (s) => fromTag(s.effectiveLanguage) ?? systemLang(),
      () => cachedLang() ?? systemLang(),
    ).then((next) => { if (live) { cacheLang(next); setCurrent(next); } });
    return () => { live = false; };
  }, [api]);
  if (current) setLang(current);
  useEffect(() => { if (current) document.title = t("brand"); }, [current]);
  const langChoice = useMemo(() => ({
    lang: current ?? systemLang(),
    adopt: (tag?: string) => { const next = fromTag(tag); if (next) { cacheLang(next); setCurrent(next); } },
  }), [current]);

  // Each look is its own chunk, fetched the first time it is shown. A saved
  // id that no longer ships, or a chunk that fails to load, falls back to
  // the default.
  const show = useCallback(async (id: string, remember: boolean) => {
    const look = looks.find((l) => l.id === id) ?? looks.find((l) => l.id === SHIPPED)!;
    setLoading(look.id);
    try {
      const next = await look.load();
      setVariant(next);
      if (remember) saveLook(next.id);
    } catch (error) {
      if (look.id === SHIPPED) throw error;
      await show(SHIPPED, true);
    } finally {
      setLoading((l) => (l === look.id ? undefined : l));
    }
  }, []);

  useEffect(() => { void show(savedLook(SHIPPED), false); }, [show]);

  const appearance = useMemo(
    () => (variant ? { current: variant.id, looks, loading, choose: (id: string) => { void show(id, true); } } : null),
    [variant, loading, show],
  );
  if (!variant || !current) return null;
  return (
    <LangContext.Provider value={langChoice}>
      <AppearanceContext.Provider value={appearance}>
        <App variant={variant} />
      </AppearanceContext.Provider>
    </LangContext.Provider>
  );
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ApiContext.Provider value={defaultApi()}>
      <Root />
    </ApiContext.Provider>
  </StrictMode>,
);
