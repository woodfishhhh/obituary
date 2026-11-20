// Links inside the site resolve against the landing page's base (/ on both
// builds). The docs are a separate build at <base>docs/ that writes
// one .html file per page.
export function siteLink(href: string): string {
  if (/^[a-z]+:/i.test(href) || href.startsWith("#")) return href;
  const [page, fragment] = href.split("#");
  const path = page.startsWith("docs/") && !page.endsWith("/") && !page.endsWith(".html") ? `${page}.html` : page;
  return `${import.meta.env.BASE_URL}${path}${fragment === undefined ? "" : `#${fragment}`}`;
}
