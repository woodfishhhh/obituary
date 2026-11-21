import { createContentLoader } from 'vitepress';

// The post's first picture, as a path under blog/ plus its true size, alt text and caption: the list's cover and the article's banner.
export function firstImage(src) {
  const figure = src.match(/<figure>[\s\S]*?<\/figure>/)?.[0] ?? src;
  const tag = figure.match(/<img\b[^>]*>/)?.[0];
  const attr = n => tag?.match(new RegExp(`\\s${n}="([^"]*)"`))?.[1];
  const [file, width, height] = [attr('src'), Number(attr('width')), Number(attr('height'))];
  return file && width > 0 && height > 0 ? { src:file.replace(/^(\.\.\/)+/, ''), width, height, alt:attr('alt') ?? '', caption:figure.match(/<figcaption>([^<]*)<\/figcaption>/)?.[1].trim() ?? '' } : null;
}

export function postIndex(pages) {
  const posts = pages.map(({ url, src, frontmatter: f }) => {
    const en = url.startsWith('/en/');
    const slug = url.split('/').pop().replace(/\.html$/, '');
    const date = f.developmentDate;
    const seq = f.sequence ?? 0;
    if (!f.title || !f.description || typeof f.draft !== 'boolean' || f.translationKey !== slug ||
        !/^\d{4}-\d{2}-\d{2}$/.test(date) || new Date(date).toISOString().slice(0, 10) !== date ||
        !Number.isInteger(seq) || seq < 0) {
      throw new Error(`Invalid post frontmatter: ${url} (see blog/README.md)`);
    }
    const words = [...new Intl.Segmenter(en ? 'en' : 'zh', {granularity:'word'}).segment(src)].filter(w => w.isWordLike).length;
    return { url, en, title:f.title, description:f.description, date, seq, draft:f.draft, key:slug, minutes:Math.max(1, Math.ceil(words / 200)), cover:firstImage(src) };
  });
  for (const p of posts) {
    if (!posts.some(q => q.en !== p.en && q.key === p.key && q.date === p.date && q.seq === p.seq)) throw new Error(`Missing translation with matching development date and sequence: ${p.url}`);
  }
  // Newest first: later development dates, then later positions within the same day, then slug.
  return posts.sort((a, b) => b.date.localeCompare(a.date) || b.seq - a.seq || a.key.localeCompare(b.key));
}

export const loadPosts = () => createContentLoader(['posts/*.md', 'en/posts/*.md'], { includeSrc:true, transform:postIndex });
