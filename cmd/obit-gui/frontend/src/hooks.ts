import { createContext, useContext, useEffect, useState } from "react";
import { errorText } from "./api";

// Every action bumps the version; pages and the summary reload on it.
export const RefreshContext = createContext<{ version: number; bump: () => void }>({ version: 0, bump: () => {} });
export const useRefresh = () => useContext(RefreshContext);

export function useLoad<T>(load: () => Promise<T>, deps: unknown[]) {
  const [state, setState] = useState<{ data?: T; error?: string; loading: boolean }>({ loading: true });
  useEffect(() => {
    let live = true;
    setState((s) => ({ ...s, loading: true }));
    load().then(
      (data) => { if (live) setState({ data, loading: false }); },
      (error) => { if (live) setState({ error: errorText(error), loading: false }); },
    );
    return () => { live = false; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);
  return state;
}
