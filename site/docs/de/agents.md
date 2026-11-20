# Agenten anbinden

## Claude Code

```powershell
obit init -g                         # all projects (~/.claude); obit init alone installs into this project (.claude/)
obit init -g --show --agent claude   # what is installed
obit init -g --uninstall             # remove only what Obituary added
```

`obit init` trägt `obit hook claude` in `settings.json` ein: für SessionStart, vor jedem Werkzeugaufruf, nach Schreibvorgängen sowie bei Stop und SessionEnd. Es schreibt eine `OBIT.md` für den Agenten und ergänzt `CLAUDE.md` um einen Verweis. Für das Benutzerkonto (`-g`) steht `@OBIT.md` in `~/.claude/CLAUDE.md`, für das Projekt steht `@.claude/OBIT.md` in dessen `CLAUDE.md`.

- Deine Einstellungen und die Reihenfolge der Schlüssel bleiben erhalten. Erneutes Ausführen erzeugt keine Duplikate. Hast du `obit.exe` verschoben, wird dabei der Pfad aktualisiert.
- Vor jeder Änderung wird eine Sicherung angelegt: beim ersten Mal `.bak`, danach `.bak.1`, `.bak.2` und so weiter. Bestehende Sicherungen werden nie überschrieben.
- Schlägt ein Hook fehl, arbeitet der Agent ohne Unterbrechung weiter. Der Fehler steht in `hook.log`. Mit `OBIT_DISABLED=1` kannst du die Hooks vorübergehend abschalten.

## Codex

```powershell
obit init -g --agent codex   # ~/.codex/hooks.json plus a block in AGENTS.md; obit init --agent codex installs into this project
```

- **Codex führt nur Hooks aus, denen du vertraust.** Öffne nach der Installation in Codex `/hooks` und markiere die Hooks von Obituary als vertrauenswürdig. Für projektspezifische Hooks musst du auch dem Projekt selbst vertrauen.
- Windows und WSL verwenden dieselbe `hooks.json`. WSL-Pfade der Form `/mnt/<drive>/…` werden zu `<drive>:\…`. Pfade im eigenen Dateisystem von WSL (`/home/…`) werden vorerst übersprungen.
- Codex bearbeitet Dateien mit `apply_patch`. Obituary liest den Patch und erfasst hinzugefügte, geänderte und verschobene Dateien. Gelöschte Dateien zählen nicht.

## Andere Agenten

```powershell
obit init -g --show                         # every supported agent, how far each integration goes, what is installed
obit init -g --agent gemini,cursor          # install one or several; without -g, into this project only
obit init -g --agent gemini --mcp --skill   # also register the MCP server and install the Skill (where supported)
obit init -g --agent gemini --uninstall     # remove only what Obituary added
```

Ohne `--agent` wird nur Claude Code eingerichtet. Obituary installiert sich nicht von selbst in anderen Agenten.

| Methode | Agenten |
|---|---|
| Native Hooks | GitHub Copilot (VS Code und Copilot CLI), Cursor, Gemini CLI, Factory Droid, Trae / Trae CN, Kimi, Mistral Vibe, Google Antigravity; Tencent CodeBuddy (CLI und IDE), Qoder / Qoder CN / Lingma, Zhipu ZCode |
| Plugin (Obituary installiert ein kleines Plugin, das `obit hook` aufruft) | OpenCode, OpenClaw, Pi, Oh My Pi, Hermes, Cline, Kilo Code, MiniMax Code, DeepSeek Harness |
| Nur Regeldatei und MCP | Roo Code, Windsurf, WorkBuddy, QoderWork |

::: tip Welche Funktionen die Agenten unterstützen
Registrierung nach Schreibvorgängen, Hinweise beim Sitzungsstart, Löschschutz, MCP und Skill sind nicht bei allen Agenten verfügbar. Die [Übersicht im Repository](https://github.com/woodfishhhh/obituary/blob/main/docs/agents.md) nennt für jede Funktion „unterstützt“, „eingeschränkt“ oder „nicht unterstützt“ und erklärt den Grund.
:::

Reicht die öffentliche Dokumentation nicht aus, um eine Konfiguration sicher zu ändern, gibt `obit init` die vollständige manuelle Anleitung aus und meldet, dass die Installation noch manuelle Schritte erfordert. Es wird nichts fälschlich als installiert gemeldet.

## MCP-Server und Agent Skill

```powershell
obit init -g --mcp --skill                 # Claude Code; add --agent codex for Codex. Neither is installed by default
```

- **`obit mcp`** ist ein MCP-Server für die Standardein- und -ausgabe (stdio). Der Agent kann `obit_mark`, `obit_mktemp`, `obit_bury`, `obit_resurrect`, `obit_extend`, `obit_spare` sowie die Abfragen `obit_list`, `obit_graveyard`, `obit_proposals` und `obit_stats` aufrufen.
- **Deine Entscheidungen bleiben bei dir.** Es gibt keine MCP-Werkzeuge, um Vorschläge zu genehmigen oder abzulehnen, `.obitrules` zu vertrauen, Einträge im Friedhof dauerhaft zu löschen oder die Konfiguration zu ändern. Der Agent kann einen Vorschlag nur an dich weitergeben.
- Der **Agent Skill** ist eine `SKILL.md`. Sie erklärt, wann der Agent seinen Dateien eine Lebensdauer geben soll, wie er sie wiederherstellbar löscht und welche Entscheidungen dir vorbehalten sind. Sobald du die Datei bearbeitest, überschreibt oder entfernt Obituary sie nicht mehr.
- Starte den Agenten nach der Installation neu. `obit doctor` prüft, ob beide Komponenten installiert sind und auf die aktuelle `obit.exe` verweisen.
