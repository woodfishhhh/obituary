import { type Variant, georgia, brand, mono, segoe, serif } from "../../theme";
import Shell from "./index";

const variant: Variant = {
  id: "tunnel",
  name: "K 隧道",
  note: "3D 时间隧道：一道道 logo 墓碑形的门框退向远处，近处金色、远处天蓝；每个文件悬在它剩余时间对应的深度，越近越快到期；滚轮在时间里穿行，延长的文件会退回深处，复活的墓碑飞出隧道",
  nav: "custom",
  home: "list",
  dark: true,
  Shell,
  tokensEn: { "--display": georgia },
  tokens: {
    ...brand,
    "--bg": "#EEF2FA", "--panel": "#FFFFFF", "--panel-2": "#F2F5FB", "--hover": "#EEF2FA",
    "--line": "#E0E6F1", "--line-2": "#C5CEE0",
    "--text": "#0E1530", "--text-2": "#34405F", "--muted": "#667089",
    "--ink": "#0E1530", "--ink-2": "#4D5878",
    "--accent": "#0048E8", "--on-accent": "#FFFFFF", "--accent-soft": "#E6EEFF",
    "--danger": "#0E1530", "--danger-soft": "#E8EBF2", "--warn": "#5C5000", "--warn-soft": "#FFF8C4", "--ok": "#0048E8",
    "--on-dark": "#F4F7FF", "--on-dark-2": "rgba(226, 234, 255, 0.74)",
    "--deep-1": "#0A3CC8", "--deep-2": "#061F78", "--deep-3": "#020A2C", "--vp-glow": "rgba(196, 224, 255, 0.62)",
    "--glass-dark": "rgba(255, 255, 255, 0.1)", "--glass-edge-dark": "rgba(255, 255, 255, 0.16)", "--hud": "rgba(2, 10, 44, 0.42)", "--top-shade": "rgba(2, 10, 44, 0.75)",
    "--glint": "#CFE6FF", "--face-top": "#FFFFFF", "--face-low": "#EEF4FF", "--face-line": "#E1E8F5",
    "--stone-shadow": "rgba(0, 6, 40, 0.5)", "--star-shadow": "rgba(120, 100, 0, 0.35)", "--halo": "rgba(248, 224, 0, 0.55)",
    "--scrim": "rgba(2, 10, 44, 0.5)",
    "--radius": "12px", "--radius-sm": "9px",
    "--shadow": "0 1px 2px rgba(14, 21, 48, 0.06), 0 0 0 1px rgba(14, 21, 48, 0.05)", "--shadow-pop": "0 24px 60px rgba(0, 6, 40, 0.45)",
    "--font": segoe, "--display": serif, "--mono": mono, "--fs": "14px", "--row": "11px",
  },
};

export default variant;
