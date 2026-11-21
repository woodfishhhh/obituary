import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

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
const intro = {
  zh: '以下表格由当前源码构建的 `obit --help` 输出生成。',
  'zh-TW': '以下表格由目前原始碼建置的 `obit --help` 輸出產生。',
  ja: '以下の表は、現在のソースから構築した `obit --help` の出力をもとに生成されています。',
  ko: '아래 표는 현재 소스에서 빌드한 `obit --help` 출력으로 생성됩니다.',
  es: 'Las tablas siguientes se generan con la salida de `obit --help` construida desde el código actual.',
  fr: 'Les tableaux suivants sont générés à partir de la sortie de `obit --help` construite avec le code actuel.',
  de: 'Die folgenden Tabellen werden aus der mit dem aktuellen Quelltext gebauten Ausgabe von `obit --help` erzeugt.',
  'pt-BR': 'As tabelas abaixo são geradas pela saída de `obit --help` compilada a partir do código atual.',
  ru: 'Следующие таблицы создаются по выводу `obit --help`, собранному из текущего исходного кода.',
};
const fallback = {
  zh: '此参数的当前行为由源码生成；请同时查看命令行 help。',
  'zh-TW': '此參數的目前行為由原始碼產生；請同時查看命令列 help。',
  ja: 'このパラメーターの現在の動作はソースから生成されます。コマンドラインの help も参照してください。',
  ko: '이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요.',
  es: 'El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda.',
  fr: 'Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide.',
  de: 'Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe.',
  'pt-BR': 'O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda.',
  ru: 'Текущее поведение параметра берётся из исходного кода; см. также справку.',
};
const common = {
  help: { zh: '显示帮助。', 'zh-TW': '顯示說明。', ja: 'ヘルプを表示します。', ko: '도움말을 표시합니다.', es: 'Muestra la ayuda.', fr: 'Affiche l’aide.', de: 'Zeigt die Hilfe an.', 'pt-BR': 'Mostra a ajuda.', ru: 'Показать справку.' },
  json: { zh: '以 JSON 输出。', 'zh-TW': '以 JSON 輸出。', ja: 'JSON 形式で出力します。', ko: 'JSON으로 출력합니다.', es: 'Muestra la salida en JSON.', fr: 'Affiche la sortie en JSON.', de: 'Gibt JSON aus.', 'pt-BR': 'Exibe a saída em JSON.', ru: 'Вывести результат в JSON.' },
  'dry-run': { zh: '只显示将执行的操作，不修改文件或服务。', 'zh-TW': '只顯示將執行的操作，不修改檔案或服務。', ja: '実行内容だけを表示し、ファイルやサービスを変更しません。', ko: '수행할 작업만 표시하고 파일이나 서비스를 변경하지 않습니다.', es: 'Solo muestra lo que haría, sin cambiar archivos ni servicios.', fr: 'Affiche seulement les actions prévues, sans modifier les fichiers ni les services.', de: 'Zeigt nur die geplanten Aktionen und ändert keine Dateien oder Dienste.', 'pt-BR': 'Mostra apenas as ações previstas, sem alterar arquivos ou serviços.', ru: 'Только показать действия без изменения файлов и служб.' },
  yes: { zh: '确认执行，不再询问。', 'zh-TW': '確認執行，不再詢問。', ja: '確認して実行し、再確認しません。', ko: '확인하고 실행하며 다시 묻지 않습니다.', es: 'Confirma la operación sin volver a preguntar.', fr: 'Confirme l’opération sans demander à nouveau.', de: 'Bestätigt den Vorgang ohne weitere Nachfrage.', 'pt-BR': 'Confirma a operação sem perguntar novamente.', ru: 'Подтвердить действие без дополнительного вопроса.' },
  version: { zh: '显示版本。', 'zh-TW': '顯示版本。', ja: 'バージョンを表示します。', ko: '버전을 표시합니다.', es: 'Muestra la versión.', fr: 'Affiche la version.', de: 'Zeigt die Version an.', 'pt-BR': 'Mostra a versão.', ru: 'Показать версию.' },
};
const removeItemUsage = readFileSync(join(repo, 'internal', 'cli', 'removeitem', 'remove-item.go'), 'utf8').split('\n').find(line => line.includes('Use:'))?.split('"')[1];
const exampleLabels = { zh: '示例：', 'zh-TW': '範例：', ja: '例：', ko: '예시:', es: 'Ejemplos:', fr: 'Exemples :', de: 'Beispiele:', 'pt-BR': 'Exemplos:', ru: 'Примеры:' };

function run(exe, args) {
  return execFileSync(exe, args, { cwd: repo, encoding: 'utf8', env: { ...process.env, OBIT_HOME: join(tmpdir(), 'obit-docs-home'), OBIT_CONFIG_DIR: join(tmpdir(), 'obit-docs-config'), OBIT_USER_HOME: join(tmpdir(), 'obit-docs-user'), OBIT_LANG: 'en' } });
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
  const available = text.split(/^Available Commands:\s*$/m)[1]?.split(/^Flags:\s*$/m)[0] ?? '';
  for (const line of available.split('\n')) {
    const match = line.match(/^\s{2,}([\w-]+)\s{2,}(.+)$/);
    if (match) children.push(match[1]);
  }
  const examples = [];
  const exampleText = text.split(/^Examples:\s*$/m)[1]?.split(/^Flags:\s*$/m)[0] ?? '';
  for (const line of exampleText.split('\n')) if (line.trim().startsWith('obit ')) examples.push(line.trim());
  return { path, usage, aliases, summary, children, examples, flags: parseFlags(text) };
}
function collect(exe) {
  const result = [];
  const visit = path => {
    const text = run(exe, path.length ? ['help', ...path] : ['help']);
    const command = parseHelp(text, path);
    if (path.length === 1 && path[0] === 'remove-item' && removeItemUsage) command.usage = 'obit ' + removeItemUsage + ' [flags]';
    result.push(command);
    for (const child of command.children) visit([...path, child]);
  };
  visit([]);
  return result.filter(command => command.path.length);
}
function description(flag, locale) {
  const key = flag.long.slice(2);
  return common[key]?.[locale] ?? (locale === 'en' ? flag.description : fallback[locale]);
}
function render(commands, locale) {
  const l = labels[locale];
  const lines = [start, `## ${l[0]}`, '', locale === 'en' ? 'The tables below are generated from the current `obit --help` output.' : intro[locale], ''];
  for (const command of commands) {
    lines.push(`### \`obit ${command.path.join(' ')}\``);
    if (command.aliases) lines.push(`${l[1]}: ${command.aliases.split(',').map(x => `\`${x.trim()}\``).join(', ')}`);
    lines.push(`${l[2]}: \`${command.usage.replaceAll('|', '\\|')}\``, '', `| ${l[3]} | ${l[4]} | ${l[5]} | ${l[6]} |`, '| --- | --- | --- | --- |');
    for (const flag of command.flags) lines.push(`| \`${flag.long}\` | ${flag.short ? `\`-${flag.short}\`` : l[7]} | \`${flag.defaultValue}\` | ${description(flag, locale)} |`);
    if (!command.flags.length) lines.push(`| ${l[7]} | ${l[7]} | ${l[7]} | ${locale === 'en' ? command.summary : fallback[locale]} |`);
    if (command.examples.length) lines.push('', locale === 'en' ? 'Examples:' : exampleLabels[locale], '', '```text', ...command.examples, '```');
    lines.push('');
  }
  lines.push(end);
  return lines.join('\n');
}
function replaceBlock(source, block) {
  const re = new RegExp(`${start}[\\s\\S]*?${end}`);
  return re.test(source) ? source.replace(re, block) : `${source.trimEnd()}\n\n${block}\n`;
}
function main() {
  const check = process.argv.includes('--check');
  const { dir, exe } = build();
  try {
    const commands = collect(exe);
    for (const locale of Object.keys(labels)) {
      const file = join(docs, locale === 'zh' ? 'commands.md' : join(locale, 'commands.md'));
      const source = readFileSync(file, 'utf8');
      const expected = replaceBlock(source, render(commands, locale));
      if (check && source !== expected) throw new Error(`command reference is stale: ${file}`);
      if (!check && source !== expected) writeFileSync(file, expected);
    }
    console.log(`Generated command references for ${commands.length} commands x ${Object.keys(labels).length} locales.`);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}
main();
