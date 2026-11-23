import { readdir, readFile } from 'node:fs/promises';
import { join, relative, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parse, NodeTypes } from '@vue/compiler-dom';
import { transform } from 'lightningcss';

function collect(html, anchors, links) {
  function visit({ tag, props, children }) {
    if (props) {
      const attrs = new Map(props.filter(p => p.type === NodeTypes.ATTRIBUTE).map(p => [p.name, p.value?.content]));
      if (attrs.has('id')) anchors.add(attrs.get('id'));
      if (tag === 'a' && attrs.has('name')) anchors.add(attrs.get('name'));
      if (attrs.has('href') && (tag === 'a' || tag === 'area' ||
        (tag === 'link' && ['alternate', 'canonical'].includes(attrs.get('rel'))))) {
        links.add(attrs.get('href') ?? '');
      }
    }
    if (tag !== 'template' && Array.isArray(children)) children.forEach(visit);
  }
  visit(parse(html));
}

export async function findBrokenLinks(output, site) {
  const root = fileURLToPath(output);
  const files = new Map();
  for (const entry of await readdir(output, { recursive: true, withFileTypes: true })) {
    if (!entry.isFile()) continue;
    const file = join(entry.parentPath, entry.name);
    const path = relative(root, file).split(sep).join('/');
    files.set(path, /\.(html|css)$/.test(path) ? await readFile(file, 'utf8') : null);
  }
  return checkLinks(files, site);
}

export function checkLinks(files, site) {
  const targets = new Map();
  const pages = [];
  for (const [file, content] of files) {
    const path = decodeURIComponent(site.pathname) + file;
    if (!file.endsWith('.html')) {
      targets.set(path, null);
      if (file.endsWith('.css') && content !== null) {
        const links = new Set();
        transform({ filename: file, code: Buffer.from(content), visitor: { Url({ url }) {
          if (!url.startsWith("#")) links.add(url);
        } } });
        pages.push([path, links, true]);
      }
      continue;
    }
    const anchors = new Set(), links = new Set();
    collect(content, anchors, links);
    targets.set(path, anchors);
    pages.push([path.replace(/\/index\.html$/, '/'), links]);
  }
  const broken = [];
  for (const [page, links, stylesheet] of pages) {
    const source = new URL(encodeURI(page), site);
    for (const href of links) {
      const url = new URL(href, source);
      if (url.protocol !== site.protocol || url.origin !== site.origin) continue;
      const path = decodeURIComponent(url.pathname);
      if (stylesheet) {
        if (!targets.has(path)) broken.push(page + " -> " + href + " (missing target)");
        continue;
      }
      const target = [path, `${path.replace(/\/$/, '')}/index.html`].find(p => targets.has(p));
      const anchor = decodeURIComponent(url.hash.slice(1));
      if (!target) broken.push(`${page} -> ${href} (missing target)`);
      else if (anchor && anchor.toLowerCase() !== 'top' && targets.get(target)?.has(anchor) === false) {
        broken.push(`${page} -> ${href} (missing anchor)`);
      }
    }
  }
  return broken.sort();
}
