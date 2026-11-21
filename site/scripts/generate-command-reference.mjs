import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { checkCatalog, translationTemplate } from './command-reference-translations.mjs';

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const docs = join(repo, 'site', 'docs');
const start = '<!-- generated-command-reference:start -->';
const end = '<!-- generated-command-reference:end -->';
const labels = {
  zh: ['参数参考', '别名', '用法', '参数', '简写', '默认值', '说明', '无'],
  en: ['Parameter reference', 'Aliases', 'Usage', 'Option', 'Short', 'Default', 'Description', 'none'],
  'zh-TW': ['參數參考', '別名', '用法', '參數', '簡寫', '預設值', '說明', '無'],
  ja: ['パラメーターリファレンス', '別名', '使い方', 'パラメーター', '短縮', '既定値', '説明', 'なし'],
  ko: ['매개변수 참고', '별칭', '사용법', '매개변수', '단축', '기본값', '설명', '없음'],
  es: ['Referencia de parámetros', 'Alias', 'Uso', 'Parámetro', 'Corto', 'Valor predeterminado', 'Descripción', 'ninguno'],
  fr: ['Référence des paramètres', 'Alias', 'Usage', 'Paramètre', 'Court', 'Valeur par défaut', 'Description', 'aucun'],
  de: ['Parameterreferenz', 'Aliase', 'Verwendung', 'Parameter', 'Kurz', 'Standardwert', 'Beschreibung', 'keiner'],
  'pt-BR': ['Referência de parâmetros', 'Aliases', 'Uso', 'Parâmetro', 'Curto', 'Padrão', 'Descrição', 'nenhum'],
  ru: ['Справочник параметров', 'Псевдонимы', 'Использование', 'Параметр', 'Кратко', 'По умолчанию', 'Описание', 'нет'],
};
const removeItemUsage = readFileSync(join(repo, 'internal', 'cli', 'removeitem', 'remove-item.go'), 'utf8').split('\n').find(line => line.includes('Use:'))?.split('"')[1];
const removeItemSummary = readFileSync(join(repo, 'internal', 'cli', 'removeitem', 'remove-item.go'), 'utf8').split(String.fromCharCode(10)).find(line => line.includes('Long:'))?.split(String.fromCharCode(96))[1];
const exampleLabels = { zh: '示例：', 'zh-TW': '範例：', ja: '例：', ko: '예시:', es: 'Ejemplos:', fr: 'Exemples :', de: 'Beispiele:', 'pt-BR': 'Exemplos:', ru: 'Примеры:' };

function run(exe, args) {
  const dir = dirname(exe);
  return execFileSync(exe, args, { cwd: dir, encoding: 'utf8', env: { ...process.env, OBIT_HOME: join(dir, 'data'), OBIT_CONFIG_DIR: join(dir, 'config'), OBIT_USER_HOME: join(dir, 'user'), OBIT_LANG: 'en', DO_NOT_TRACK: '1' } }).replaceAll('\r\n', '\n');
}
function build() {
  const dir = mkdtempSync(join(tmpdir(), 'obit-docs-build-'));
  const exe = join(dir, process.platform === 'win32' ? 'obit.exe' : 'obit');
  execFileSync('go', ['build', '-o', exe, './cmd/obit'], { cwd: repo, stdio: 'inherit' });
  return { dir, exe };
}
function parseFlags(text) {
  const section = text.split(/^Flags:\s*$/m)[1] ?? '';
  const result = [];
  for (const line of section.split('\n')) {
    const match = line.match(/^\s{2,}(?:-([^,]),\s+)?(--[\w-]+)(?:\s+(\S+))?\s{2,}(.+)$/);
    if (!match) continue;
    const [, short = '', long, type = 'bool', raw] = match;
    const defaultMatch = raw.match(/\(default ([^)]+)\)/);
    let value = defaultMatch?.[1] ?? '';
    if (!value) value = type === 'bool' ? 'false' : type === 'int' ? '0' : type === 'string' ? '""' : '';
    result.push({ long, short, defaultValue: value, description: raw.replace(/\s*\(default [^)]+\)/, '').trim() });
  }
  return result;
}
function parseHelp(text, path) {
  const usage = text.match(/^Usage:\s*\n\s*(.+)$/m)?.[1]?.trim() ?? `obit ${path.join(' ')} [flags]`;
  const aliases = text.match(/^Aliases:\s*\n\s*(.+)$/m)?.[1]?.trim() ?? '';
  const summary = text.split('\n')[0]?.trim() ?? '';
  const children = [];
  const available = text.match(/^Available Commands:\n((?:  [^\n]*\n)+)/m)?.[1] ?? '';
  for (const line of available.split('\n')) {
    const match = line.match(/^  ([\w-]+)(?:\s|$)/);
    if (match) children.push(match[1]);
  }
  const examples = [];
  const exampleText = text.match(/^Examples:\n((?:  [^\n]*\n)+)/m)?.[1] ?? '';
  for (const line of exampleText.trim().split('\n').filter(Boolean)) {
    const [code, comment] = line.trim().split(/\s+#\s+(.+)/);
    examples.push({ code, comment });
  }
  return { name: ['obit', ...path].join(' '), usage, aliases, summary, children, examples, flags: parseFlags(text) };
}
function collect(exe) {
  const result = [];
  const visit = path => {
    const text = run(exe, path.length ? ['help', ...path] : ['help']);
    const command = parseHelp(text, path);
    // This documented background entry point is hidden from Cobra's child list.
    if (command.name === 'obit hook' && !command.children.includes('finish')) command.children.push('finish');
    if (path.length === 1 && path[0] === 'remove-item' && removeItemUsage) command.usage = 'obit ' + removeItemUsage + ' [flags]';
    if (path.length === 1 && path[0] === 'remove-item' && removeItemSummary) command.summary = removeItemSummary;
    result.push(command);
    for (const child of command.children) visit([...path, child]);
  };
  visit([]);
  return result;
}
function render(commands, catalog, locale) {
  const l = labels[locale];
  const lines = [start, `## ${l[0]}`, ''];
  for (const command of commands) {
    const message = (key, source) => locale === 'en' ? source : catalog[command.name][key].translations[locale];
    lines.push(`### \`${command.name}\``, '', message('summary', command.summary), '');
    if (command.aliases) lines.push(`${l[1]}: ${command.aliases.split(',').map(x => `\`${x.trim()}\``).join(', ')}`);
    lines.push(`${l[2]}: \`${command.usage}\``, '', `| ${l[3]} | ${l[4]} | ${l[5]} | ${l[6]} |`, '| --- | --- | --- | --- |');
    for (const flag of command.flags) lines.push(`| \`${flag.long}\` | ${flag.short ? `\`-${flag.short}\`` : l[7]} | \`${flag.defaultValue}\` | ${message(flag.long, flag.description).replaceAll('|', '\\|')} |`);
    if (command.examples.length) lines.push('', locale === 'en' ? 'Examples:' : exampleLabels[locale], '', '```text', ...command.examples.map(({ code, comment }) => comment ? `${code}  # ${message(`example: ${code}`, comment)}` : code), '```');
    lines.push('');
  }
  lines.push(end);
  return lines.join('\n');
}
function replaceBlock(source, block) {
  const re = new RegExp(`${start}[\\s\\S]*?${end}`);
  return re.test(source) ? source.replace(re, () => block) : `${source.trimEnd()}\n\n${block}\n`;
}
function main() {
  const check = process.argv.includes('--check');
  const englishOnly = process.argv.includes('--english-only');
  const { dir, exe } = build();
  try {
    const commands = collect(exe);
    const translatedLocales = Object.keys(labels).filter(l => l !== 'en');
    const expectedCatalog = translationTemplate(commands, translatedLocales);
    if (process.argv.includes('--template')) {
      console.log(JSON.stringify(expectedCatalog, null, 2));
      return;
    }
    const catalog = JSON.parse(readFileSync(new URL('./command-reference-translations.json', import.meta.url), 'utf8'));
    checkCatalog(expectedCatalog, catalog, englishOnly ? [] : translatedLocales);
    const locales = englishOnly ? ['en'] : Object.keys(labels);
    for (const locale of locales) {
      const file = join(docs, locale === 'zh' ? 'commands.md' : join(locale, 'commands.md'));
      const source = readFileSync(file, 'utf8');
      const expected = replaceBlock(source, render(commands, catalog, locale));
      if (check && source !== expected) throw new Error(`command reference is stale: ${file}`);
      if (!check && source !== expected) writeFileSync(file, expected);
    }
    console.log(`${check ? 'Checked' : 'Generated'} ${commands.length} commands × ${locales.length} locales.`);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}
main();
