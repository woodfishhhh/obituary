import { createContext, useContext } from "react";
import type { Api } from "./types";
import { createMockApi } from "./mock";

declare global {
  interface Window {
    go?: { gui?: { API?: Api } };
    runtime?: { ClipboardSetText?: (text: string) => Promise<boolean> };
  }
}

// Inside obit-gui the Wails binding exists; in `pnpm dev` and the playground
// the in-memory mock stands in for it.
export function defaultApi(): Api {
  return window.go?.gui?.API ?? createMockApi("normal");
}

export const ApiContext = createContext<Api>(createMockApi("normal"));
export const useApi = () => useContext(ApiContext);

export function errorText(error: unknown): string {
  if (typeof error === "string") return error;
  if (error instanceof Error) return error.message;
  return String(error);
}
