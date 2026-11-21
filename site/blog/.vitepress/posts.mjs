import { createContentLoader } from 'vitepress';

export function postIndex(pages) {
  const posts = pages.map(({ url, src, frontmatter: f }) => {
    const en = url.startsWith('/en/');
    const slug = url.split('/').pop().replace(/\.html$/, '');
    const date = f.developmentDate;
    if (!f.title || !f.description || typeof f.draft !== 'boolean' || f.translationKey !== slug ||
        !/^\d{4}-\d{2}-\d{2}$/.test(date) || new Date(date).toISOString().slice(0, 10) !== date) {
      throw new Error(`Invalid post frontmatter: ${url} (see blog/README.md)`);
    }
    const words = [...new Intl.Segmenter(en ? 'en' : 'zh', {granularity:'word'}).segment(src)].filter(w => w.isWordLike).length;
    return { url, en, title:f.title, description:f.description, date, draft:f.draft, key:slug, minutes:Math.max(1, Math.ceil(words / 200)) };
  });
  for (const p of posts) {
    if (!posts.some(q => q.en !== p.en && q.key === p.key && q.date === p.date)) throw new Error(`Missing translation with matching development date: ${p.url}`);
  }
  return posts.sort((a, b) => b.date.localeCompare(a.date) || a.key.localeCompare(b.key));
}

export const loadPosts = () => createContentLoader(['posts/*.md', 'en/posts/*.md'], { includeSrc:true, transform:postIndex });
