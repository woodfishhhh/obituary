// Every word the blog's own pages show, in the two languages the blog is written in.
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

// A development date (YYYY-MM-DD) as the parts the pages print. Read as UTC so the server and the browser agree.
export function dateParts(iso: string, en: boolean) {
  const d = new Date(`${iso}T00:00:00Z`);
  const [year, m, day, wd] = [d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate(), d.getUTCDay()];
  return en
    ? { year, month: MONTHS[m], day, weekday: WEEKDAYS[wd], short: `${MONTHS[m]} ${day}`, long: `${MONTHS[m]} ${day}, ${year}` }
    : { year, month: `${m + 1} 月`, day, weekday: `周${'日一二三四五六'[wd]}`, short: `${m + 1} 月 ${day} 日`, long: `${year} 年 ${m + 1} 月 ${day} 日` };
}

const zh = {
  skip: '跳到正文', mainNav: '主导航', brand: '讣告', section: '开发博客', docs: '文档', dev: '开发文档', themeToLight: '切换到浅色模式', themeToDark: '切换到深色模式',
  heroTitle: ['讣告是怎么', '做出来', '的'],
  first: '从第一篇读起', rss: '订阅 RSS',
  count: (n: number) => `${n} 篇文章`, total: (n: number) => `共约 ${n} 分钟阅读`,
  latest: '最新', timeline: '时间线', timelineHint: '按开发日期，从近到远', nth: (n: number) => `第 ${n} 篇`, minutes: (n: number) => `阅读约 ${n} 分钟`, draft: '草稿', read: '阅读',
  back: '全部文章', pager: '文章导航', prev: '上一篇', next: '下一篇',
  legal: 'MIT 许可。截图里的文件都是示例数据。', home: '官网首页',
  lostTitle: '这一页没有找到', lostText: '链接可能写错了，或者这篇文章换了地址。',
};
const en: typeof zh = {
  skip: 'Skip to content', mainNav: 'Main navigation', brand: 'Obituary', section: 'Development blog', docs: 'Docs', dev: 'Developer guide', themeToLight: 'Switch to light mode', themeToDark: 'Switch to dark mode',
  heroTitle: ['How Obituary got ', 'built', ''],
  first: 'Start from the first post', rss: 'Subscribe via RSS',
  count: (n) => `${n} posts`, total: (n) => `About ${n} min of reading`,
  latest: 'Latest', timeline: 'Timeline', timelineHint: 'By development date, newest first', nth: (n) => `Post ${n}`, minutes: (n) => `${n} min read`, draft: 'Draft', read: 'Read',
  back: 'All posts', pager: 'Article navigation', prev: 'Previous', next: 'Next',
  legal: 'MIT licensed. The files in the screenshots are sample data.', home: 'Home',
  lostTitle: 'This page was not found', lostText: 'The link may be mistyped, or the post may have moved.',
};
export const copy = (isEn: boolean) => (isEn ? en : zh);
