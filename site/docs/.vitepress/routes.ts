import { locales } from '../../landing/src/locales';

export function pageSuffix(path: string) {
  const prefix = locales.find(l => l.path && path.startsWith(l.path))?.path ?? '';
  return path.slice(prefix.length).replace(/(^|\/)index\.md$/, '$1').replace(/\.md$/, '.html');
}
