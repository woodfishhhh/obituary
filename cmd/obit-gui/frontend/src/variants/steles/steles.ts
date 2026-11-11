import { type Variant, brand, mono, segoe, serif } from "../../theme";
import Shell from "./index";

const variant: Variant = {
  id: "steles",
  name: "G 碑林",
  note: "logo 里的墓碑本来就是一张折角的文件：每个文件立成一块蓝金描边的碑，墓地里草越长越高，长到碑顶那天火化；即将下葬的碑还没立，轮廓上的光就是寿命",
  nav: "custom",
  home: "list",
  dark: false,
  Shell,
  tokens: {
    ...brand,
    "--bg": "#F1F4FA", "--panel": "#FFFFFF", "--panel-2": "#E9EEF7", "--hover": "#EEF2FA",
    "--line": "#E0E6F1", "--line-2": "#C5CEE0",
    "--text": "#0E1530", "--text-2": "#34405F", "--muted": "#667089",
    "--accent": "#0048E8", "--on-accent": "#FFFFFF", "--accent-soft": "#E6EEFF",
    "--danger": "#0E1530", "--danger-soft": "#E8EBF2", "--warn": "#5C5000", "--warn-soft": "#FFF8C4", "--ok": "#0048E8",
    "--rail": "#E6EBF5", "--rail-hover": "#DCE3F1",
    "--glint": "#CFE6FF", "--face-top": "#FFFFFF", "--face-low": "#EFF4FF", "--fold": "#FFF1A0",
    "--glass": "rgba(255, 255, 255, 0.6)", "--stone-shadow": "rgba(0, 40, 140, 0.16)", "--star-shadow": "rgba(120, 100, 0, 0.35)",
    "--ground-a": "rgba(0, 120, 248, 0.3)", "--ground-b": "rgba(248, 224, 0, 0.22)", "--terrace": "#D6DDEB",
    "--scrim": "rgba(14, 21, 48, 0.36)",
    "--radius": "14px", "--radius-sm": "9px",
    "--shadow": "0 1px 2px rgba(14, 21, 48, 0.06), 0 0 0 1px rgba(14, 21, 48, 0.05)", "--shadow-pop": "0 18px 48px rgba(0, 40, 140, 0.2), 0 0 0 1px rgba(14, 21, 48, 0.05)",
    "--font": segoe, "--display": serif, "--mono": mono, "--fs": "14px", "--row": "11px",
  },
};

export default variant;
