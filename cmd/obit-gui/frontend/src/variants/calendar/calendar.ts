import { type Variant, brand, din, mono } from "../../theme";
import Shell from "./index";

const variant: Variant = {
  id: "calendar",
  name: "I 日历",
  note: "命运日历：每一列是一天，文件排在它入土或火化的那一天；日期下面那条从钴蓝到金色的带子就是时间；把文件拖到后面的某一天就延长到那天",
  nav: "custom",
  home: "list",
  dark: false,
  Shell,
  tokens: {
    ...brand,
    "--bg": "#FFFFFF", "--panel": "#FFFFFF", "--panel-2": "#F3F5FA", "--hover": "#F2F4F9",
    "--line": "#E6E9F0", "--line-2": "#D0D6E2",
    "--text": "#111522", "--text-2": "#3A4256", "--muted": "#6E768A",
    "--accent": "#0048E8", "--on-accent": "#FFFFFF", "--on-accent-2": "#C9D8FF", "--accent-soft": "#EAF0FF",
    "--danger": "#111522", "--danger-soft": "#ECEEF3", "--warn": "#5C5000", "--warn-soft": "#FFF8C4", "--ok": "#0048E8",
    "--glint": "#B9D9FF", "--cream": "#FCEF8A",
    "--today": "#F6F9FF", "--weekend": "#FAFBFC", "--target": "#F9FBFF",
    "--chip-shadow": "rgba(17, 21, 34, 0.05)", "--chip-on-shadow": "rgba(0, 72, 232, 0.28)",
    "--scrim": "rgba(17, 21, 34, 0.32)",
    "--radius": "12px", "--radius-sm": "8px",
    "--shadow": "0 1px 2px rgba(17, 21, 34, 0.08), 0 0 0 1px rgba(17, 21, 34, 0.04)", "--shadow-pop": "0 16px 44px rgba(17, 21, 34, 0.18), 0 0 0 1px rgba(17, 21, 34, 0.05)",
    "--font": din, "--mono": mono, "--fs": "14px", "--row": "10px",
  },
};

export default variant;
