import { type Variant, brand, manrope, mono } from "../../theme";
import Shell from "../../shells/Classic";

const variant: Variant = {
  id: "paper",
  name: "C 纸白",
  note: "极简：纯白、黑字、细线、小字号，像 Vercel 和 Notion；蓝色只出现在选中和主按钮上",
  nav: "sidebar",
  home: "list",
  dark: false,
  Shell,
  tokens: {
    ...brand,
    "--bg": "#FFFFFF", "--panel": "#FFFFFF", "--panel-2": "#FAFAFA", "--hover": "#F5F5F5",
    "--line": "#EFEFEF", "--line-2": "#E3E3E3",
    "--text": "#111111", "--text-2": "#333333", "--muted": "#7A7A7A",
    "--accent": "#0048E8", "--on-accent": "#FFFFFF", "--accent-soft": "#F0F4FE",
    "--danger": "#D92D20", "--danger-soft": "#FEF3F2", "--warn": "#B54708", "--warn-soft": "#FFFAEB", "--ok": "#067647",
    "--nav-bg": "#FAFAFA", "--nav-text": "#111111", "--nav-muted": "#6B6B6B", "--nav-active": "#EFEFEF", "--nav-active-text": "#111111", "--nav-line": "#EFEFEF",
    "--radius": "6px", "--radius-sm": "6px",
    "--shadow": "0 0 0 1px #EFEFEF", "--shadow-pop": "0 12px 40px rgba(0,0,0,0.12), 0 0 0 1px #E3E3E3",
    "--font": manrope, "--mono": mono, "--fs": "13px", "--row": "9px",
  },
};

export default variant;
