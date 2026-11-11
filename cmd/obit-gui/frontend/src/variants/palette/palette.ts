import { type Variant, brand, mono, segoe } from "../../theme";
import Shell from "./index";

const variant: Variant = {
  id: "palette",
  name: "J 一行",
  note: "像 Everything 和 PowerToys Run：从托盘叫出来就是一行输入，压在 Windows 11 亚克力玻璃上；页面变成这一行的范围，回车复活或延长，底栏写明每个快捷键",
  nav: "custom",
  home: "list",
  dark: false,
  Shell,
  tokens: {
    ...brand,
    "--bg": "#E6EBF5", "--panel": "#FFFFFF", "--panel-2": "#F4F6FB", "--hover": "rgba(14, 21, 48, 0.05)",
    "--line": "rgba(14, 21, 48, 0.08)", "--line-2": "rgba(14, 21, 48, 0.16)",
    "--text": "#10162A", "--text-2": "#3A4359", "--muted": "#6A7286",
    "--accent": "#0048E8", "--on-accent": "#FFFFFF", "--accent-soft": "rgba(0, 72, 232, 0.09)", "--accent-line": "rgba(0, 72, 232, 0.25)",
    "--danger": "#10162A", "--danger-soft": "rgba(14, 21, 48, 0.06)", "--warn": "#5C5000", "--warn-soft": "#FFF8C4", "--ok": "#0048E8",
    "--glass": "rgba(250, 251, 254, 0.64)", "--glass-edge": "rgba(255, 255, 255, 0.8)", "--bar": "rgba(243, 245, 250, 0.72)",
    "--row-active": "rgba(0, 72, 232, 0.07)", "--chip": "rgba(14, 21, 48, 0.06)", "--gold-ink": "#B8A300",
    "--light-cobalt": "rgba(0, 72, 232, 0.55)", "--light-sky": "rgba(0, 120, 248, 0.36)", "--light-gold": "rgba(248, 224, 0, 0.7)",
    "--scrim": "rgba(16, 22, 42, 0.28)",
    "--radius": "8px", "--radius-sm": "6px",
    "--shadow": "0 0 0 1px rgba(14, 21, 48, 0.06)", "--shadow-pop": "0 20px 50px rgba(0, 30, 110, 0.18), 0 2px 6px rgba(0, 30, 110, 0.06)",
    "--font": segoe, "--display": '"Segoe UI Variable Display", "Segoe UI", "Microsoft YaHei UI", sans-serif', "--mono": mono, "--fs": "14px", "--row": "10px",
  },
};

export default variant;
