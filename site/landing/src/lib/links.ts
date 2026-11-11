// Links inside the site resolve against the landing page's base (/obituary/
// on GitHub Pages). The docs are a separate build at <base>docs/ that writes
// one .html file per page.
export function siteLink(href: string): string {
  if (/^[a-z]+:/i.test(href) || href.startsWith("#")) return href;
  const path = href.startsWith("docs/") && !href.endsWith("/") && !href.endsWith(".html") ? `${href}.html` : href;
  return `${import.meta.env.BASE_URL}${path}`;
}
