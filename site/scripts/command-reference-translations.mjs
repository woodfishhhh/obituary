// null means pending. Commands without a comment are shell syntax, not translation entries.
export function translationTemplate(commands, locales) {
  const entry = source => ({ source, translations: Object.fromEntries(locales.map(l => [l, null])) });
  return Object.fromEntries(commands.map(({ name, summary, flags, examples }) => [name, {
    summary: entry(summary),
    ...Object.fromEntries(flags.map(f => [f.long, entry(f.description)])),
    ...Object.fromEntries(examples.filter(e => e.comment).map(e => [`example: ${e.code}`, entry(e.comment)])),
  }]));
}

export function checkCatalog(expected, catalog, locales) {
  const pending = [];
  for (const [command, fields] of Object.entries(expected)) {
    for (const [key, { source }] of Object.entries(fields)) {
      const row = catalog[command]?.[key];
      if (!row) throw new Error(`Missing translation entry: ${command} / ${key}`);
      if (row.source !== source) throw new Error(`English help changed: ${command} / ${key}\n  recorded: ${row.source}\n  current:  ${source}`);
      for (const locale of locales) {
        if (!row.translations?.[locale]?.trim()) pending.push(`${command} / ${key} (${locale})`);
      }
    }
  }
  for (const [command, fields] of Object.entries(catalog)) {
    for (const key of Object.keys(fields)) {
      if (!expected[command]?.[key]) throw new Error(`Obsolete translation entry: ${command} / ${key}`);
    }
  }
  if (pending.length) throw new Error(`${pending.length} missing translations; first: ${pending[0]}`);
}
