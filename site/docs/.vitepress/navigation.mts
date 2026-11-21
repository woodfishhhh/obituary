import { readFileSync } from 'node:fs';
import { type DefaultTheme, type LocaleConfig } from 'vitepress';
import { locales, type Lang } from '../../landing/src/locales';
import { guides } from './agents/catalog';

// Navigation titles come from the translated page itself.
const groups = [
  ['index', 'how-it-works'], ['agents', 'auto-register', 'delete-guard', 'grave-notices'],
  ['folder-policies', 'docker', 'desktop'], ['commands', 'config', 'uninstall', 'troubleshooting', 'faq', 'code-signing', 'privacy'],
];
const labels: Record<Lang, string[]> = {
  zh: ['首页','文档','下载','开始','和 agent 一起用','更多','参考','本页内容','上一篇','下一篇','搜索','语言','目录','回到顶部','在 GitHub 上修改这一页'],
  'zh-TW': ['首頁','文件','下載','開始','與代理程式一起使用','更多','參考','本頁內容','上一篇','下一篇','搜尋','語言','目錄','回到頂端','在 GitHub 上編輯此頁'],
  en: ['Home','Docs','Download','Start','With your agents','More','Reference','On this page','Previous','Next','Search','Language','Menu','Back to top','Edit this page on GitHub'],
  ja: ['ホーム','ドキュメント','ダウンロード','はじめに','エージェントと使う','その他の機能','リファレンス','このページの内容','前へ','次へ','検索','言語','目次','先頭に戻る','GitHub で編集'],
  ko: ['홈','문서','다운로드','시작하기','에이전트와 함께 사용하기','기타 기능','참고 자료','이 페이지의 목차','이전','다음','검색','언어','메뉴','맨 위로','GitHub에서 이 페이지 편집'],
  es: ['Inicio','Documentación','Descargar','Primeros pasos','Uso con agentes','Más funciones','Referencia','En esta página','Anterior','Siguiente','Buscar','Idioma','Menú','Volver arriba','Editar esta página en GitHub'],
  fr: ['Accueil','Documentation','Télécharger','Premiers pas','Utiliser les agents','Autres fonctions','Référence','Sur cette page','Précédent','Suivant','Rechercher','Langue','Menu','Haut de page','Modifier cette page sur GitHub'],
  de: ['Startseite','Dokumentation','Download','Erste Schritte','Mit Agenten arbeiten','Weitere Funktionen','Referenz','Auf dieser Seite','Zurück','Weiter','Suchen','Sprache','Menü','Nach oben','Diese Seite auf GitHub bearbeiten'],
  'pt-BR': ['Início','Documentação','Baixar','Primeiros passos','Uso com agentes','Outros recursos','Referência','Nesta página','Anterior','Próxima','Pesquisar','Idioma','Menu','Voltar ao topo','Editar esta página no GitHub'],
  ru: ['Главная','Документация','Скачать','Начало работы','Работа с агентами','Другие возможности','Справочник','На этой странице','Назад','Далее','Поиск','Язык','Меню','Наверх','Редактировать на GitHub'],
};

const devLabels: Record<Lang, string> = {zh:'开发', en:'Development', 'zh-TW':'開發', ja:'開発', ko:'개발', es:'Desarrollo', fr:'Développement', de:'Entwicklung', 'pt-BR':'Desenvolvimento', ru:'Разработка'};
const devPages = ['index','architecture','repository','build-test','gui','safety','agent-integration','i18n','release','docs-and-blog'];
const blogLabels: Record<Lang,string> = {zh:'博客',en:'Blog','zh-TW':'部落格',ja:'ブログ',ko:'블로그',es:'Blog',fr:'Blog',de:'Blog','pt-BR':'Blog',ru:'Блог'};

export function docLocales(site: string, repo: string): LocaleConfig<DefaultTheme.Config> {
  return Object.fromEntries(locales.map(({ id, path, html, name }) => {
    const [home,docs,download,start,agents,more,reference,outline,prev,next,,language,menu,top,edit] = labels[id];
    const sidebar = groups.map((pages, i) => ({ text: [start,agents,more,reference][i], items: pages.map(page => {
      const text = readFileSync(new URL(`../${path}${page}.md`, import.meta.url), 'utf8').match(/^# (.+)$/m)![1].trim();
      const link = `/${path}${page === 'index' ? '' : page}`;
      return page === 'agents' ? { text, link, collapsed: true, items: guides.map(a => ({ text: a.name, link: `${link}/${a.id}` })) } : { text, link };
    }) }));
    const brand = id === 'zh' ? '讣告' : id === 'zh-TW' ? '訃告' : 'Obituary';
    const dev = id === 'zh' ? '/dev/' : '/en/dev/';
    const section = id === 'zh' || id === 'en' ? { [dev]: [{ text: devLabels[id], items: devPages.map(page => ({
      text: readFileSync(new URL(`..${dev}${page}.md`, import.meta.url), 'utf8').match(/^# (.+)$/m)![1].trim(),
      link: dev + (page === 'index' ? '' : page),
    })) }] } : {};
    return [id === 'zh' ? 'root' : id, {
      label: name, lang: html, title: `${brand} · ${docs}`, titleTemplate: `:title · ${brand}`,
      description: `${docs} · ${brand}: ${sidebar[0].items.map(p => p.text).join(' · ')}`,
      themeConfig: { siteTitle: brand, logoLink: site+path, sidebar: { [`/${path}`]: sidebar, ...section },
        nav: [{text:home,link:site+path,target:'_self',noIcon:true}, sidebar[0].items[0], {text:docs,link:`/${path}how-it-works`}, {text:devLabels[id],link:dev}, {text:blogLabels[id],link:site+(id === 'zh' ? 'blog/' : 'blog/en/'),target:'_self',noIcon:true}, {text:download,link:`${repo}/releases`}],
        outline: {label:outline,level:[2,3]}, docFooter:{prev,next}, langMenuLabel:language, sidebarMenuLabel:menu, returnToTopLabel:top,
        editLink:{pattern:`${repo}/edit/main/site/docs/:path`, text:edit},
        footer: {message:'MIT', copyright:brand},
        ...(id === 'zh' ? {
          darkModeSwitchLabel:'外观', lightModeSwitchTitle:'切换到浅色', darkModeSwitchTitle:'切换到深色',
          notFound:{title:'这一页不在这里',quote:'它可能被改名或挪走了。',linkLabel:'回到文档首页',linkText:'回到文档首页'},
          footer:{message:'以 MIT 许可证开源。',copyright:'讣告 Obituary'},
        } : {}),
      },
    }];
  }));
}

export const searchLocales = Object.fromEntries(locales.map(({id}) => [id === 'zh' ? 'root' : id, {
  translations: {button:{buttonText:labels[id][10],buttonAriaLabel:labels[id][10]},
    ...(id === 'zh' ? {modal:{displayDetails:'显示详细列表',resetButtonTitle:'清除',backButtonTitle:'关闭搜索',noResultsText:'没有找到',footer:{selectText:'选择',navigateText:'切换',closeText:'关闭'}}} : {})},
}]));
