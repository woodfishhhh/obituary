// The sample files the window demo shows. Names, folders and sizes are what an agent really leaves
// behind; the only words in here are the free-text notes, which have a table per language.
import type { Burial, Item } from "@gui/types";
import type { Lang } from "@/locales";

const SHOP = "D:\\work\\shop";
const API = "D:\\work\\api-server";
const DL = "C:\\Users\\me\\Downloads";
const HOUR = 3600e3;
const DAY = 24 * HOUR;
const MB = 1024 * 1024;
const GB = 1024 * MB;

export type NoteKey = "backup" | "docs" | "recording" | "session" | "build" | "spec" | "failing";

// Notes the user or an agent typed in, in the language of the page; the window around them has
// Chinese and English only.
export const notes: Record<Lang, Record<NoteKey, string>> = {
  zh: {
    backup: "改之前的备份",
    docs: "周五前整理进文档",
    recording: "复现步骤，确认后可删",
    session: "agent 会话留下的",
    build: "能随时重新生成",
    spec: "等评审结束",
    failing: "失败用例的现场",
  },
  en: {
    backup: "backup before the edit",
    docs: "fold into the docs by Friday",
    recording: "repro steps, delete once confirmed",
    session: "left behind by an agent session",
    build: "can be regenerated any time",
    spec: "until the review is done",
    failing: "state of the failing run",
  },
  "zh-TW": {
    backup: "修改前的備份",
    docs: "週五前整理進文件",
    recording: "重現步驟，確認後可刪",
    session: "agent 工作階段留下的",
    build: "隨時能重新產生",
    spec: "等審查結束",
    failing: "失敗測試的現場",
  },
  ja: {
    backup: "編集前のバックアップ",
    docs: "金曜までにドキュメントへまとめる",
    recording: "再現手順、確認できたら削除",
    session: "エージェントのセッションが残したもの",
    build: "いつでも再生成できる",
    spec: "レビューが終わるまで",
    failing: "失敗したテストの現場",
  },
  ko: {
    backup: "편집 전 백업",
    docs: "금요일까지 문서에 정리",
    recording: "재현 단계, 확인되면 삭제",
    session: "에이전트 세션이 남긴 것",
    build: "언제든 다시 생성 가능",
    spec: "리뷰가 끝날 때까지",
    failing: "실패한 실행의 상태",
  },
  es: {
    backup: "copia antes de editar",
    docs: "pasar a la documentación antes del viernes",
    recording: "pasos para reproducir; borrar al confirmar",
    session: "dejado por una sesión del agente",
    build: "se puede regenerar cuando sea",
    spec: "hasta que acabe la revisión",
    failing: "estado de la ejecución fallida",
  },
  fr: {
    backup: "sauvegarde avant la modification",
    docs: "à intégrer à la doc d’ici vendredi",
    recording: "étapes de reproduction, à supprimer une fois confirmé",
    session: "laissé par une session d’agent",
    build: "peut être régénéré à tout moment",
    spec: "jusqu’à la fin de la relecture",
    failing: "état de l’exécution en échec",
  },
  de: {
    backup: "Backup vor der Änderung",
    docs: "bis Freitag in die Doku einarbeiten",
    recording: "Schritte zum Nachstellen, nach Bestätigung löschen",
    session: "von einer Agent-Sitzung hinterlassen",
    build: "jederzeit neu erzeugbar",
    spec: "bis das Review durch ist",
    failing: "Zustand des fehlgeschlagenen Laufs",
  },
  "pt-BR": {
    backup: "backup antes da edição",
    docs: "incluir na documentação até sexta",
    recording: "passos para reproduzir; apagar depois de confirmar",
    session: "deixado por uma sessão do agente",
    build: "pode ser gerado de novo a qualquer momento",
    spec: "até a revisão terminar",
    failing: "estado da execução que falhou",
  },
  ru: {
    backup: "резервная копия перед правкой",
    docs: "свести в документацию до пятницы",
    recording: "шаги воспроизведения, удалить после подтверждения",
    session: "осталось после сессии агента",
    build: "можно пересобрать в любой момент",
    spec: "до конца ревью",
    failing: "состояние упавшего запуска",
  },
};

type Source = "rule" | "cli" | "watch" | "proposal";
// day: 0 is today; hour is the time of day it expires (today's rows count from now instead).
interface Row { path: string; dir?: boolean; day: number; hour: number; life: number; source: Source; note?: string; key?: NoteKey }

const rows: Row[] = [
  { path: `${SHOP}\\shot.png`, day: 0, hour: 0.4, life: 24, source: "rule" },
  { path: `${SHOP}\\build.log`, day: 0, hour: 4, life: 72, source: "rule", note: "rule *.log (builtin:4)" },
  { path: `${SHOP}\\scratch.py`, day: 0, hour: 8, life: 48, source: "rule", note: "rule *.py (.obitrules:3)" },
  { path: `${SHOP}\\debug.log`, day: 1, hour: 2, life: 72, source: "rule", note: "rule *.log (builtin:4)" },
  { path: `${SHOP}\\notes.md`, day: 1, hour: 9, life: 24, source: "cli", key: "docs" },
  { path: `${SHOP}\\out.png`, day: 1, hour: 13, life: 48, source: "rule" },
  { path: `${SHOP}\\e2e-report`, dir: true, day: 1, hour: 18, life: 72, source: "rule", note: "rule playwright-report/ (builtin:7)" },
  { path: `${API}\\trace.json`, day: 2, hour: 3, life: 48, source: "cli" },
  { path: `${API}\\dump.sql`, day: 2, hour: 10, life: 72, source: "cli" },
  { path: `${SHOP}\\repro.sh`, day: 2, hour: 16, life: 96, source: "cli", key: "recording" },
  { path: `${SHOP}\\coverage`, dir: true, day: 3, hour: 9, life: 96, source: "rule", note: "rule coverage/ (builtin:5)" },
  { path: `${API}\\agent.jsonl`, day: 3, hour: 13, life: 72, source: "cli", key: "session" },
  { path: `${SHOP}\\.env.bak`, day: 3, hour: 17, life: 72, source: "rule", note: "rule *.bak (builtin:2)", key: "backup" },
  { path: `${DL}\\setup.msi`, day: 4, hour: 11, life: 168, source: "cli" },
  { path: `${SHOP}\\test-out`, dir: true, day: 4, hour: 14, life: 96, source: "rule", key: "failing" },
  { path: `${API}\\plan.bak`, day: 4, hour: 18, life: 120, source: "rule", note: "rule *.bak (builtin:2)", key: "backup" },
  { path: `${SHOP}\\dist`, dir: true, day: 5, hour: 10, life: 168, source: "proposal", note: "approved #29 (rule dist/ (builtin:6))", key: "build" },
  { path: `${API}\\heap.dmp`, day: 5, hour: 14, life: 120, source: "cli" },
  { path: `${DL}\\demo.mkv`, day: 5, hour: 18, life: 168, source: "cli" },
  { path: `${SHOP}\\.next\\cache`, dir: true, day: 6, hour: 9, life: 168, source: "rule", note: "rule .next/ (builtin:8)" },
  { path: `${DL}\\report.csv`, day: 6, hour: 14, life: 336, source: "cli" },
  { path: `${API}\\seed.json`, day: 6, hour: 19, life: 96, source: "cli" },
  { path: `${SHOP}\\tmp`, dir: true, day: 7, hour: 9, life: 168, source: "rule", note: "rule tmp/ (builtin:3)" },
  { path: `${API}\\old.sql`, day: 7, hour: 13, life: 168, source: "rule", note: "rule *.sql.bak (builtin:2)", key: "backup" },
  { path: `${SHOP}\\lint.log`, day: 7, hour: 19, life: 168, source: "rule", note: "rule *.log (builtin:4)" },
  { path: `${DL}\\spec-v7.pdf`, day: 9, hour: 9, life: 336, source: "cli", key: "spec" },
  { path: `${API}\\.cache`, dir: true, day: 11, hour: 9, life: 336, source: "rule", note: "rule .cache/ (builtin:1)" },
];

export interface DemoItem extends Item { key?: NoteKey }

// Every expiry is built from the clock at render time, so "today" is always today.
export function demoItems(lang: Lang, now = Date.now()): DemoItem[] {
  const midnight = new Date(now).setHours(0, 0, 0, 0);
  return rows.map((r, i) => {
    // Today's rows stay today, even late in the evening, so no day is crowded by spillover.
    const expires = r.day === 0 ? Math.max(now + 5 * 60e3, Math.min(now + r.hour * HOUR, midnight + DAY - (10 + 20 * (2 - i)) * 60e3)) : midnight + r.day * DAY + r.hour * HOUR;
    return {
      id: 21 + i,
      path: r.path,
      isDir: !!r.dir,
            createdAt: new Date(expires - r.life * HOUR).toISOString(),
      expiresAt: new Date(expires).toISOString(),
      source: r.source,
      note: r.key ? notes[lang][r.key] : r.note,
      key: r.key,
    };
  });
}

// size in bytes, hours since it was buried, hours until it is cremated
const graves: [string, boolean, number, number, number][] = [
  [`${SHOP}\\.venv`, true, 412 * MB, 96, 72],
  [`${SHOP}\\.vite`, true, 186 * MB, 40, 128],
  [`${DL}\\IMG_0931.HEIC`, false, 3.2 * MB, 60, 108],
  [`${SHOP}\\debug.log`, false, 1.8 * MB, 150, 18],
  [`${API}\\heap.snap`, false, 842 * MB, 30, 138],
  [`${DL}\\obs-1030.mkv`, false, 2.4 * GB, 120, 48],
  [`${SHOP}\\cart.png`, false, 612 * 1024, 20, 148],
  [`${SHOP}\\old-cache`, true, 1.1 * GB, 144, 24],
  [`${API}\\core.dmp`, false, 318 * MB, 80, 88],
  [`${SHOP}\\lock.bak`, false, 412 * 1024, 12, 156],
  [`${SHOP}\\render-7.png`, false, 4.7 * MB, 5, 163],
  [`${DL}\\setup-20.msi`, false, 30 * MB, 100, 68],
];

export function demoGraves(now = Date.now()): Burial[] {
  return graves.map(([path, isDir, size, since, left], i) => ({
    id: 90 - i,
    path,
    isDir,
    size: Math.round(size),
    buriedAt: new Date(now - since * HOUR).toISOString(),
    purgeAt: new Date(now + left * HOUR).toISOString(),
  }));
}

// What Obituary has cleared so far, shown at the top of Settings.
export const lifetime = { buried: 1284, buriedBytes: Math.round(165.36 * GB), purgedBytes: Math.round(158.9 * GB), resurrected: 17 };
