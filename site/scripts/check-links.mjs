import { readdir, readFile } from 'node:fs/promises';
import { join, relative, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parse, NodeTypes } from '@vue/compiler-dom';

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
    files.set(path, path.endsWith('.html') ? await readFile(file, 'utf8') : null);
  }
  return checkLinks(files, site);
}

export function checkLinks(files, site) {
  const targets = new Map();
  const pages = [];
  for (const [file, html] of files) {
    const path = decodeURIComponent(site.pathname) + file;
    if (html === null) {
      targets.set(path, null);
      continue;
    }
    const anchors = new Set(), links = new Set();
    collect(html, anchors, links);
    targets.set(path, anchors);
    pages.push([path.replace(/\/index\.html$/, '/'), links]);
  }
  const broken = [];
  for (const [page, links] of pages) {
    const source = new URL(encodeURI(page), site);
    for (const href of links) {
      const url = new URL(href, source);
      if (url.protocol !== site.protocol || url.origin !== site.origin) continue;
      const path = decodeURIComponent(url.pathname);
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
