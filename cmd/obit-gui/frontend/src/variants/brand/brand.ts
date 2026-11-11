import { type Variant, brand, manrope, mono } from "../../theme";
import Shell from "../../shells/Classic";

const variant: Variant = {
  id: "brand",
  name: "D 蓝金",
  note: "品牌最浓的一版：钴蓝顶栏、金色选中线、瓷白内容区，每一项都有从蓝到金的寿命线",
  nav: "topbar",
  home: "list",
  dark: false,
  Shell,
  tokens: {
    ...brand,
    "--bg": "#F7F8FB", "--panel": "#FFFFFF", "--panel-2": "#F2F4F9", "--hover": "#EEF1F8",
    "--line": "#E4E8F1", "--line-2": "#D3D9E6",
    "--text": "#0E1530", "--text-2": "#2C3554", "--muted": "#68708A",
    "--accent": "#0048E8", "--on-accent": "#FFFFFF", "--accent-soft": "#E7EEFD",
    "--danger": "#C9302C", "--danger-soft": "#FCEBEA", "--warn": "#9A6700", "--warn-soft": "#FFF5D1", "--ok": "#127A3F",
    "--nav-bg": "#0B1D57", "--nav-text": "#FFFFFF", "--nav-muted": "#AFC3F5", "--nav-active": "transparent", "--nav-active-text": "#FFFFFF", "--nav-line": "#F8E000",
    "--radius": "10px", "--radius-sm": "7px",
    "--shadow": "0 0 0 1px #E4E8F1, 0 1px 2px rgba(14,21,48,0.05)", "--shadow-pop": "0 18px 48px rgba(11,29,87,0.22)",
    "--font": manrope, "--mono": mono, "--fs": "14px", "--row": "11px",
  },
};

export default variant;
