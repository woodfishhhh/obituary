# Befehle

Ersetze `<id>` durch die Nummer aus der Ausgabe von `obit add` oder `obit bury` bzw. aus der Liste von `obit list` oder `obit graveyard`. Die Zeiteinheiten `s`, `m`, `h`, `d` und `w` lassen sich kombinieren, etwa zu `1d12h`.

## Lebensdauer

```powershell
obit add debug.log 2h                       # bury it in 2 hours (alias: mark)
obit add output\shots 1d12h                 # folders work too
obit add report.zip --at "2026-12-01 18:00" # at a given moment
obit list                                   # what is left and for how long
obit extend <id> 1d                         # one more day
obit spare <id>                             # stop tracking, keep the file (alias: cancel)
obit reap                                   # bury everything that is due now (alias: sweep; --dry-run previews)
```

## Der Friedhof

```powershell
obit bury old-build                         # bury right away
obit graveyard                              # what is in the graveyard and when it will be purged (--here: this folder only)
obit resurrect <id>                         # put it back; if the name is taken, it comes back renamed, never overwriting
obit resurrect --last                       # undo the last deletion: everything one obit bury / obit rm buried comes back
obit purge                                  # purge what is past retention (--all for everything, after confirming; --dry-run previews)
obit purge <id>                             # purge this grave now (permanent; asks first, --yes outside a terminal)
```

Bei `resurrect` und `purge` gilt eine Zahl ohne Pfad (`5` oder `#5`) immer als ID. Eine unbekannte ID führt zu einem Fehler und wird nicht als Dateiname interpretiert. Für eine Datei mit numerischem Namen gib `./5` oder einen absoluten Pfad an.

## Wrapper für Löschbefehle

```powershell
obit rm -rf build            # rm's common options; every path is buried
obit rmdir empty-dir
obit git-clean -fdx
obit remove-item .\build     # one literal path only
```

## Regeln und Vorschläge

```powershell
obit rules                       # the rules in effect here
obit rules check debug.log out/  # preview what would be registered and by which rule
obit rules apply debug.log       # register by rule
obit trust                       # trust this project's .obitrules (--yes when non-interactive)
obit proposals                   # proposals waiting for you, and what was registered without asking
obit approve 3                   # approve
obit reject 3                    # reject
$dir = obit mktemp 1d            # a temporary folder that buries itself
```

## Ordnerrichtlinien und Docker

```powershell
obit watch C:\Users\you\Downloads 2w
obit watch --list
obit watch --remove C:\Users\you\Downloads
obit docker add web-1 3d
obit docker list
```

## Integration und Hintergrundbetrieb

```powershell
obit init                       # Claude Code hooks and Skill, context menu, Start menu and desktop shortcuts; the tray starts at login
obit init -g --agent codex      # Codex
obit init -g --guard-rm         # delete guard
obit init -g --mcp              # add the MCP server
obit init --no-context-menu --no-shortcuts --no-skill   # skip the context menu, shortcuts or Skill
obit init -g --show             # what is installed
obit daemon enable              # tray at login plus the fallback scheduled task
obit daemon status
obit daemon disable
```

## Abfragen und Prüfungen

```powershell
obit doctor              # read-only check (--json)
obit stats               # eulogy: how much was buried, space freed, resurrections (alias: eulogy)
obit log                 # the full record of everything done
obit config              # show the configuration; --create writes a commented default
obit config telemetry off # disable usage reporting and clear the local installation ID; update checks are separate
obit hook audit          # with OBIT_HOOK_AUDIT=1: what each hook received and did
obit uninstall           # remove everything Obituary set up (try --dry-run first)
```

Abfragen und Befehle mit Ergebnis (`list`, `graveyard`, `proposals`, `stats`, `mark`, `bury`, `resurrect` und weitere) unterstützen `--json` für Skripte und Agenten. Lösch-Wrapper wie `obit rm` behalten das Ausgabeformat des ursprünglichen Befehls bei und unterstützen `--json` nicht.

## Umgebungsvariablen

| Variable | Effekt |
|---|---|
| `OBIT_DISABLED=1` | Hooks vorübergehend abschalten |
| `OBIT_TELEMETRY=0`, `DO_NOT_TRACK=1` | Nutzungsstatistiken für diesen Prozess abschalten; CI sendet keine Meldungen |
| `OBIT_GUARD_RM=0` | Löschschutz vorübergehend ausschalten |
| `OBIT_HOOK_AUDIT=1` | Aufzeichnen, was jeder Hook empfangen und getan hat |
| `OBIT_TRUST_PROJECT_RULES=1` | Der Datei `.obitrules` des Projekts in CI vertrauen |
| `OBIT_HOME`, `OBIT_CONFIG_DIR`, `OBIT_USER_HOME` | Die Verzeichnisse für Daten, Konfiguration und das Benutzer-Home umstellen |



## Kompatible Parameter der Löschwrapper

- git-clean akzeptiert die Git-clean-Optionen -d, -f, -n, -q, -x, -X, -e pattern, --exclude pattern und -- sowie --dry-run, --force und --quiet.
- rm akzeptiert -r, -R, -f, -d, -v, --recursive, --force, --dir, --verbose, --preserve-root und -- mit Pfaden.
- rmdir akzeptiert nur -v oder --verbose und Pfade; rmdir -p wird abgelehnt.
- Windows remove-item akzeptiert nur -Recurse, -Force, -Path oder -LiteralPath und genau einen literalen Pfad. Für Wildcards in Bash obit rm verwenden.

<!-- generated-command-reference:start -->
## Parameterreferenz

Die folgenden Tabellen werden aus der mit dem aktuellen Quelltext gebauten Ausgabe von `obit --help` erzeugt.

### `obit add`
Aliase: `add`, `mark`
Verwendung: `obit add <path>... <duration> \| add <path>... --at <time> [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--at` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--force` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |
| `--no-daemon` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--note` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

Beispiele:

```text
obit add debug.log 2h
obit add output\shots 1d12h
obit add report.zip --at "2026-12-01 18:00"
```

### `obit approve`
Verwendung: `obit approve <proposal id>... [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |

### `obit bury`
Verwendung: `obit bury <path>... [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--force` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |
| `--note` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit cancel`
Aliase: `cancel`, `spare`
Verwendung: `obit cancel <id\|path>... [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |

### `obit completion`
Verwendung: `obit completion [command]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |

### `obit completion bash`
Verwendung: `obit completion bash`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--no-descriptions` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit completion fish`
Verwendung: `obit completion fish [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--no-descriptions` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit completion powershell`
Verwendung: `obit completion powershell [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--no-descriptions` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit completion zsh`
Verwendung: `obit completion zsh [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--no-descriptions` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit config`
Verwendung: `obit config [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--create` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--language` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit config telemetry`
Verwendung: `obit config telemetry on\|off [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |

### `obit daemon`
Verwendung: `obit daemon [command]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |

### `obit daemon disable`
Verwendung: `obit daemon disable [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--dry-run` | keiner | `false` | Zeigt nur die geplanten Aktionen und ändert keine Dateien oder Dienste. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |

### `obit daemon enable`
Verwendung: `obit daemon enable [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--dry-run` | keiner | `false` | Zeigt nur die geplanten Aktionen und ändert keine Dateien oder Dienste. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--resident` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit daemon start`
Verwendung: `obit daemon start [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |

### `obit daemon status`
Verwendung: `obit daemon status [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |

### `obit daemon stop`
Verwendung: `obit daemon stop [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |

### `obit docker`
Verwendung: `obit docker [command]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |

### `obit docker add`
Verwendung: `obit docker add <container name\|ID> <lifespan> [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |

### `obit docker list`
Verwendung: `obit docker list [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |

### `obit doctor`
Verwendung: `obit doctor [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |
| `--verbose` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit extend`
Verwendung: `obit extend <id\|path>... <duration> [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |

Beispiele:

```text
obit extend 3 1d
obit extend debug.log 30m
```

### `obit git-clean`
Verwendung: `obit git-clean [options] [--] [pathspec]... [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |

### `obit graveyard`
Aliase: `graveyard`, `graves`
Verwendung: `obit graveyard [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--here` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--json` | keiner | `false` | Gibt JSON aus. |

### `obit help`
Verwendung: `obit help [command] [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |

### `obit hook`
Verwendung: `obit hook [command]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |

### `obit hook antigravity`
Verwendung: `obit hook antigravity [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook audit`
Verwendung: `obit hook audit [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--lines` | `-n` | `40` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook claude`
Verwendung: `obit hook claude [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook cline`
Verwendung: `obit hook cline [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook codebuddy`
Verwendung: `obit hook codebuddy [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook codebuddy-ide`
Verwendung: `obit hook codebuddy-ide [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook codex`
Verwendung: `obit hook codex [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook copilot`
Verwendung: `obit hook copilot [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook cursor`
Verwendung: `obit hook cursor [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook droid`
Verwendung: `obit hook droid [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook gemini`
Verwendung: `obit hook gemini [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook hermes`
Verwendung: `obit hook hermes [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook kilocode`
Verwendung: `obit hook kilocode [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook kimi`
Verwendung: `obit hook kimi [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook lingma`
Verwendung: `obit hook lingma [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook minimax-code`
Verwendung: `obit hook minimax-code [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook omp`
Verwendung: `obit hook omp [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook openclaw`
Verwendung: `obit hook openclaw [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook opencode`
Verwendung: `obit hook opencode [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook pi`
Verwendung: `obit hook pi [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook qoder`
Verwendung: `obit hook qoder [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook qoder-cn`
Verwendung: `obit hook qoder-cn [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook trae`
Verwendung: `obit hook trae [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook trae-cn`
Verwendung: `obit hook trae-cn [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook vibe`
Verwendung: `obit hook vibe [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit hook zcode`
Verwendung: `obit hook zcode [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--notice-only` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit init`
Verwendung: `obit init [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--agent` | keiner | `[claude]` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--context-menu` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--dry-run` | keiner | `false` | Zeigt nur die geplanten Aktionen und ändert keine Dateien oder Dienste. |
| `--format` | keiner | `"text"` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--global` | `-g` | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--guard-rm` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--mcp` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--no-context-menu` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--no-shortcuts` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--no-skill` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--show` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--skill` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--uninstall` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--yes` | keiner | `false` | Bestätigt den Vorgang ohne weitere Nachfrage. |

Beispiele:

```text
obit init -g                 # Claude Code, for every project (~/.claude)
obit init                    # Claude Code, this project only (.claude/)
obit init -g --agent codex   # Codex (~/.codex/hooks.json + AGENTS.md)
obit init -g --uninstall     # remove only what Obituary added
obit init -g --show          # what is installed
obit init --show --format markdown > docs/agents.md
```

### `obit list`
Aliase: `list`, `ls`
Verwendung: `obit list [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |

### `obit log`
Verwendung: `obit log [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |
| `--lines` | `-n` | `20` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit mcp`
Verwendung: `obit mcp [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |

### `obit mktemp`
Verwendung: `obit mktemp <lifespan> [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |
| `--name` | keiner | `"tmp"` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--session` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

Beispiele:

```text
obit mktemp 2h --name screenshots
```

### `obit proposals`
Verwendung: `obit proposals [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |

### `obit purge`
Aliase: `purge`, `cremate`
Verwendung: `obit purge [id\|original path]... [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--all` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--dry-run` | keiner | `false` | Zeigt nur die geplanten Aktionen und ändert keine Dateien oder Dienste. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |
| `--yes` | keiner | `false` | Bestätigt den Vorgang ohne weitere Nachfrage. |

### `obit reject`
Verwendung: `obit reject <proposal id>... [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |

### `obit remove-item`
Verwendung: `obit remove-item [-Recurse] [-Force] [-Path\|-LiteralPath] <path> [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |

### `obit resurrect`
Aliase: `resurrect`, `raise`
Verwendung: `obit resurrect <id\|original path>... \| --last [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |
| `--last` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit rm`
Verwendung: `obit rm [options] <path>... [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |

### `obit rmdir`
Verwendung: `obit rmdir [options] <path>... [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |

### `obit rules`
Verwendung: `obit rules [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |

### `obit rules apply`
Verwendung: `obit rules apply <path>... [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |
| `--session` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit rules check`
Verwendung: `obit rules check <path>... [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |
| `--session` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit stats`
Aliase: `stats`, `eulogy`
Verwendung: `obit stats [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |

### `obit sweep`
Aliase: `sweep`, `reap`
Verwendung: `obit sweep [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--dry-run` | keiner | `false` | Zeigt nur die geplanten Aktionen und ändert keine Dateien oder Dienste. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |

### `obit trust`
Verwendung: `obit trust [rules file] [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--yes` | keiner | `false` | Bestätigt den Vorgang ohne weitere Nachfrage. |

### `obit uninstall`
Verwendung: `obit uninstall [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--dry-run` | keiner | `false` | Zeigt nur die geplanten Aktionen und ändert keine Dateien oder Dienste. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--purge-data` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--yes` | keiner | `false` | Bestätigt den Vorgang ohne weitere Nachfrage. |

### `obit untrust`
Verwendung: `obit untrust [rules file] [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |

### `obit update`
Verwendung: `obit update [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--apply` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--check` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--relaunch-gui` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--wait-pid` | keiner | `0` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

### `obit watch`
Verwendung: `obit watch <directory> <lifespan> \| watch --list \| watch --remove <directory> [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--age` | keiner | `"first-seen"` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--clear-exclusion` | keiner | `0` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--help` | `-h` | `false` | Zeigt die Hilfe an. |
| `--json` | keiner | `false` | Gibt JSON aus. |
| `--list` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--no-daemon` | keiner | `false` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--policy` | keiner | `0` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--remove` | keiner | `""` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |
| `--remove-id` | keiner | `0` | Das aktuelle Verhalten dieses Parameters wird aus dem Quelltext erzeugt; siehe auch die Hilfe. |

<!-- generated-command-reference:end -->
