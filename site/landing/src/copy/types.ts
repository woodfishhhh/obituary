import type { zh } from "./zh";

// Every language fills in the same shape as Simplified Chinese, so a missing sentence fails the type check.
export type Strings = typeof zh;
