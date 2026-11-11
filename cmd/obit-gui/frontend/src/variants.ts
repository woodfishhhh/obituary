// Every variant, for the playground (dev only). The app itself loads only
// the shipped ones, on demand, through catalog.ts.
import type { Variant } from "./theme";
import mica from "./variants/mica/mica";
import night from "./variants/night/night";
import paper from "./variants/paper/paper";
import brand from "./variants/brand/brand";
import horizon from "./variants/horizon/horizon";
import notice from "./variants/notice/notice";
import steles from "./variants/steles/steles";
import vigil from "./variants/vigil/vigil";
import calendar from "./variants/calendar/calendar";
import palette from "./variants/palette/palette";
import tunnel from "./variants/tunnel/tunnel";

export type { Tokens, Variant } from "./theme";

export const variants: Variant[] = [mica, night, paper, brand, horizon, notice, steles, vigil, calendar, palette, tunnel];

export const byId = (id: string) => variants.find((v) => v.id === id) ?? variants[0];
