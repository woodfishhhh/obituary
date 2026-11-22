import type { ComponentType } from "react";
import { defaultsOf, type Knob, type Values } from "./knobs";
import * as bento from "./styles/Bento";
import * as canvas from "./styles/Canvas";
import * as dock from "./styles/Dock";
import * as flip from "./styles/Flip";
import * as orbit from "./styles/Orbit";
import * as prompt from "./styles/Prompt";
import * as spotlight from "./styles/Spotlight";
import * as stream from "./styles/Stream";

export interface StyleDef {
  id: string;
  no: string;
  name: string;
  en: string;
  /** One line for the owner: what this one is. */
  blurb: string;
  knobs: Knob[];
  Wall: ComponentType<{ v: Values }>;
  /** Whether the shared command bar sits under the wall (the others show the command themselves). */
  panel: boolean;
}

const def = (id: string, no: string, name: string, en: string, blurb: string, m: { knobs: Knob[]; Wall: StyleDef["Wall"] }, panel = true): StyleDef => ({ id, no, name, en, blurb, ...m, panel });

export const styles: StyleDef[] = [
  def("stream", "01", "流光带", "Stream", "还是滚动，但换成药丸标签、斜着的几条带子、两端淡出；悬停一家它就放大、整条带暂停。", stream),
  def("dock", "02", "码头", "Dock", "macOS Dock 那样：指针划过，图标按距离平滑放大，标签浮在上面。窄屏自动多排。", dock),
  def("spotlight", "03", "聚光灯", "Spotlight", "整齐的卡片网格，一束光跟着指针（或键盘焦点）走，照亮卡片和描边。", spotlight),
  def("orbit", "04", "星轨", "Orbit", "三圈轨道围着讣告的图标缓缓转，俯视倾斜；悬停整片天空停住，点一颗就选中。", orbit),
  def("bento", "05", "分舱", "Bento", "按用法分成三舱：终端里用、编辑器里用、插件与桌面助手；最常用的两家放大。（分组是我猜的，站上数据里没有。）", bento),
  def("canvas", "06", "画布", "Canvas", "没有边界的画布：拖、甩、方向键都能平移，空闲时缓缓漂；Tab 到哪一家就把它滑到中间。", canvas),
  def("flip", "07", "翻牌", "Flip", "点一张牌它翻过来，背面就是那一条命令，带复制和文档；同一时间只翻一张。", flip, false),
  def("prompt", "08", "提示符", "Prompt", "一个终端窗口：打字筛选或点选，命令一个字一个字敲出来，贴着产品本身的气质。", prompt, false),
];

export const initial = Object.fromEntries(styles.map((s) => [s.id, defaultsOf(s.knobs)])) as Record<string, Values>;
