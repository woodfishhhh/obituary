// A release's notes carry the same text in Chinese and in English, one after
// the other with a horizontal rule between. The window shows the part in its
// own language: English when there is none, and never the rule.
export type NotesLang = "zh" | "en";

// A line of three or more -, * or _ (spaces allowed between) is a rule.
const rule = /^ {0,3}([-*_])(?:[ \t]*\1){2,}[ \t]*$/;

function sections(text: string): string[] {
  const out: string[] = [];
  let current: string[] = [];
  for (const line of text.split(/\r?\n/)) {
    if (rule.test(line)) {
      out.push(current.join("\n"));
      current = [];
    } else {
      current.push(line);
    }
  }
  out.push(current.join("\n"));
  return out.map((s) => s.trim()).filter(Boolean);
}

// Chinese notes keep product and command names in Latin letters, so a part
// counts as Chinese when enough of its letters are Han characters.
function isChinese(section: string): boolean {
  const han = (section.match(/\p{Script=Han}/gu) ?? []).length;
  const latin = (section.match(/[A-Za-z]/g) ?? []).length;
  return han > 0 && han / (han + latin) > 0.1;
}

export function notesFor(text: string, lang: NotesLang): string {
  const parts = sections(text);
  if (parts.length < 2) return text.trim();
  const chinese = parts.filter(isChinese);
  const other = parts.filter((p) => !isChinese(p));
  const chosen = lang === "zh" ? (chinese.length ? chinese : other) : (other.length ? other : chinese);
  return chosen.join("\n\n");
}
