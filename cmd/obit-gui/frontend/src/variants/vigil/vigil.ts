import { type Variant, brand, mono, segoe, serif } from "../../theme";
import Shell from "./index";

const variant: Variant = {
  id: "vigil",
  name: "H 长明",
  note: "守灵的夜：深色、细线、只有金色是光。每个文件点着一盏灯，剩下的寿命就是灯的亮度；右边把一盏灯拿近看，时间用细宋体放大；延长是添油，复活时灯会亮一下",
  nav: "custom",
  home: "list",
  dark: true,
  Shell,
  tokens: {
    ...brand,
    "--bg": "#070A14", "--panel": "#0C1120", "--panel-2": "#111829", "--hover": "#0F1524",
    "--line": "#1A2236", "--line-2": "#2A3550",
    "--text": "#ECEEF5", "--text-2": "#B4BBCC", "--muted": "#7A8399",
    "--accent": "#3D8EFF", "--on-accent": "#FFFFFF", "--accent-soft": "#0D1B38",
    "--danger": "#ECEEF5", "--danger-soft": "#151C2E", "--warn": "#F8E000", "--warn-soft": "#1E1C10", "--ok": "#3D8EFF",
    "--on-gold": "#221D00",
    "--glow": "rgba(248, 224, 0, 0.5)", "--halo-gold": "rgba(248, 224, 0, 0.2)", "--halo-blue": "rgba(0, 72, 232, 0.12)",
    "--scrim": "rgba(2, 4, 10, 0.66)",
    "--radius": "3px", "--radius-sm": "2px",
    "--shadow": "0 0 0 1px #1A2236", "--shadow-pop": "0 24px 64px rgba(0, 0, 0, 0.6)",
    "--font": segoe, "--display": serif, "--mono": mono, "--fs": "14px", "--row": "11px",
  },
};

export default variant;
