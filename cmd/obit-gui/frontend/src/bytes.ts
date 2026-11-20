const units = ["B", "KB", "MB", "GB", "TB"];

// A size for people: whole bytes, one decimal up to MB, and two decimals from GB up, where a
// total such as "165.36 GB" is worth reading closely.
export function bytes(n: number): string {
  if (!n) return "0 B";
  let i = 0;
  let v = n;
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
  if (i === 0) return `${Math.round(v)} B`;
  if (i >= 3) return `${v.toFixed(2)} ${units[i]}`;
  return `${v >= 100 ? Math.round(v) : v.toFixed(1)} ${units[i]}`;
}
