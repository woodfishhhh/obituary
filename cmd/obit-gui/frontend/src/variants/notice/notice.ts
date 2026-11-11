import { type Variant, brand, georgia, mono, serif } from "../../theme";
import Shell from "./index";

const variant: Variant = {
  id: "notice",
  name: "F 讣闻",
  note: "整个窗口是报纸的讣闻版：钴蓝报头、宋体排版、分栏刊登每一座坟墓，即将下葬是一张寿数表，复活时盖金色印章",
  nav: "custom",
  home: "list",
  dark: false,
  Shell,
  tokensEn: { "--font": georgia },
  tokens: {
    ...brand,
    "--bg": "#F4F2EA", "--panel": "#FAF9F4", "--panel-2": "#ECE9DF", "--hover": "#ECE9DF",
    "--line": "#DDD9CC", "--line-2": "#B9B5A8",
    "--text": "#16181D", "--text-2": "#3E4048", "--muted": "#6F7076",
    "--accent": "#0048E8", "--on-accent": "#FFFFFF", "--accent-soft": "#E6EAF2",
    "--danger": "#16181D", "--danger-soft": "#E7E4DA", "--warn": "#4A4200", "--warn-soft": "#F6EDA8", "--ok": "#0048E8",
    "--press-muted": "#9A9C9F", "--scrim": "rgba(22, 24, 29, 0.42)",
    "--radius": "0px", "--radius-sm": "0px",
    "--shadow": "none", "--shadow-pop": "0 24px 60px rgba(22, 24, 29, 0.28)",
    "--font": serif, "--mono": mono, "--fs": "14px", "--row": "11px",
  },
};

export default variant;
