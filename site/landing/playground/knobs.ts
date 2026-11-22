// The dials of a wall: what each one is called, its range and its starting value. The playground draws the
// panel from these, and "copy parameters" hands the current values back as JSON.
export type KnobValue = number | string | boolean;
export type Values = Record<string, KnobValue>;

export type Knob =
  | { key: string; label: string; type: "range"; min: number; max: number; step: number; value: number; unit?: string }
  | { key: string; label: string; type: "choice"; options: [string, string][]; value: string }
  | { key: string; label: string; type: "toggle"; value: boolean };

export const range = (key: string, label: string, min: number, max: number, step: number, value: number, unit = ""): Knob => ({ key, label, type: "range", min, max, step, value, unit });
export const choice = (key: string, label: string, options: [string, string][], value: string): Knob => ({ key, label, type: "choice", options, value });
export const toggle = (key: string, label: string, value: boolean): Knob => ({ key, label, type: "toggle", value });

export const logoColor = choice("logo", "logo 颜色", [["color", "彩色"], ["mono", "单色"], ["hover", "悬停上色"]], "color");
export const defaultsOf = (knobs: Knob[]): Values => Object.fromEntries(knobs.map((k) => [k.key, k.value]));
