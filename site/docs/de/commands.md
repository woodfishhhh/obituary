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

Mit `obit mark --docker-volume cache 3d` (auch `add`) wird `cache` in drei Tagen begraben, mit `obit bury --docker-volume -- cache` sofort. [Docker-Container und Volumes](./docker)

## Integration und Hintergrundbetrieb

```powershell
obit init                       # Claude Code hooks and Skill, context menu, Start menu and desktop shortcuts; the tray starts at login
obit init -g --agent codex      # Codex
obit init -g --guard-rm         # delete guard
obit init -g --mcp              # add the MCP server
obit init --no-context-menu --no-shortcuts --no-skill   # skip the context menu, shortcuts or Skill
obit init -g --uninstall --no-context-menu   # on uninstall keep the context menu and remove the rest; --no-shortcuts and --no-skill work the same way
obit init -g --show             # what is installed
obit daemon enable              # tray at login plus the fallback scheduled task
obit daemon status
obit daemon disable
```

Bei aktiver Desktop-Überwachung verarbeitet der Tray-Prozess fällige Einträge sofort; ein unabhängiger Auftrag prüft stündlich nach. Ohne Tray-Überwachung für dieses Datenverzeichnis läuft der Auftrag alle fünf Minuten.

## Abfragen und Prüfungen

```powershell
obit doctor              # health check: short summary, full HTML report (--open, --verbose, --json)
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

### `obit`

Obituary (讣告), der Papierkorb für KI-Agenten: Dateien erhalten eine Lebensdauer; ist sie abgelaufen, werden sie auf einem Friedhof begraben, von dem aus sie sich wiederbeleben lassen

Verwendung: `obit [command]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu obit |
| `--version` | `-v` | `false` | Version von obit |

### `obit add`

Pfaden eine Lebensdauer geben (erneutes Hinzufügen eines verfolgten Pfads setzt dessen Ablauf zurück)

Aliase: `add`, `mark`
Verwendung: `obit add <path>... <duration> | add <path>... --at <time> [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--at` | keiner | `""` | Zu einer bestimmten lokalen Zeit ablaufen: "2006-01-02 15:04", "2006-01-02" oder "15:04" |
| `--docker-volume` | keiner | `false` | Argumente als Docker-Volume-Namen behandeln |
| `--force` | keiner | `false` | Ordner zulassen, die ein .git-Repository enthalten |
| `--help` | `-h` | `false` | Hilfe zu add |
| `--json` | keiner | `false` | JSON ausgeben |
| `--no-daemon` | keiner | `false` | Den Daemon im Hintergrund nicht starten |
| `--note` | keiner | `""` | Freitextnotiz, die in der Liste angezeigt wird |

Beispiele:

```text
obit add debug.log 2h
obit add output\shots 1d12h
obit add report.zip --at "2026-12-01 18:00"
```

### `obit approve`

Vorschläge genehmigen: registrieren, was sie auflisten

Verwendung: `obit approve <proposal id>... [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu approve |
| `--json` | keiner | `false` | JSON ausgeben |

### `obit bury`

Pfade sofort auf den Friedhof verschieben (Wiederherstellung mit obit resurrect)

Verwendung: `obit bury <path>... [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--docker-volume` | keiner | `false` | Argumente als Docker-Volume-Namen behandeln |
| `--force` | keiner | `false` | Ordner zulassen, die ein .git-Repository enthalten |
| `--help` | `-h` | `false` | Hilfe zu bury |
| `--json` | keiner | `false` | JSON ausgeben |
| `--note` | keiner | `""` | Freitextnotiz, die zusammen mit dem Begräbnis gespeichert wird |

### `obit cancel`

Pfade oder Container-IDs verschonen: die Verfolgung beenden und die Objekte behalten

Aliase: `cancel`, `spare`
Verwendung: `obit cancel <id|path>... [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu cancel |
| `--json` | keiner | `false` | JSON ausgeben |

### `obit completion`

Das Autovervollständigungsskript von obit für die angegebene Shell erzeugen.

Verwendung: `obit completion [command]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu completion |

### `obit completion bash`

Das Autovervollständigungsskript für die Bash-Shell erzeugen.

Verwendung: `obit completion bash`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu bash |
| `--no-descriptions` | keiner | `false` | Beschreibungen der Autovervollständigung deaktivieren |

### `obit completion fish`

Das Autovervollständigungsskript für die Fish-Shell erzeugen.

Verwendung: `obit completion fish [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu fish |
| `--no-descriptions` | keiner | `false` | Beschreibungen der Autovervollständigung deaktivieren |

### `obit completion powershell`

Das Autovervollständigungsskript für PowerShell erzeugen.

Verwendung: `obit completion powershell [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu powershell |
| `--no-descriptions` | keiner | `false` | Beschreibungen der Autovervollständigung deaktivieren |

### `obit completion zsh`

Das Autovervollständigungsskript für die Zsh-Shell erzeugen.

Verwendung: `obit completion zsh [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu zsh |
| `--no-descriptions` | keiner | `false` | Beschreibungen der Autovervollständigung deaktivieren |

### `obit config`

Die Konfigurationsdatei und die wirksamen Einstellungen anzeigen (--create schreibt die Standardwerte)

Verwendung: `obit config [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--create` | keiner | `false` | Eine Konfigurationsdatei mit den Standardwerten schreiben |
| `--help` | `-h` | `false` | Hilfe zu config |
| `--language` | keiner | `""` | Die Sprache festlegen: auto, en oder zh-CN |

### `obit config telemetry`

Nutzungsstatistik: Eine zufällige Installations-ID, Version, Betriebssystem, Architektur, Oberflächensprache und Installationskanal helfen, Installationszahlen und die Nutzung der Versionen zu messen. Es werden keine Dateien, Pfade oder Befehle gesendet; IP-Adressen und User-Agents werden nicht gespeichert. Deaktivieren mit obit config telemetry off oder OBIT_TELEMETRY=0.

Verwendung: `obit config telemetry on|off [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu telemetry |

### `obit daemon`

Den Daemon im Hintergrund (obitd) steuern

Verwendung: `obit daemon [command]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu daemon |

### `obit daemon disable`

Die geplante Aufgabe und den dauerhaft laufenden Daemon entfernen

Verwendung: `obit daemon disable [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--dry-run` | keiner | `false` | Nur die Änderungen ausgeben |
| `--help` | `-h` | `false` | Hilfe zu disable |

### `obit daemon enable`

Benutzerzeitplan aktivieren (mit Tray-Überwachung stündlich, sonst alle 5 Minuten); unter Windows auch das Tray-Symbol bei der Anmeldung starten

Verwendung: `obit daemon enable [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--dry-run` | keiner | `false` | Nur die Änderungen ausgeben |
| `--help` | `-h` | `false` | Hilfe zu enable |
| `--resident` | keiner | `false` | Das Tray aktivieren, ohne die geplante Aufgabe als Rückfallebene zu ändern |

### `obit daemon start`

Den Daemon jetzt starten

Verwendung: `obit daemon start [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu start |

### `obit daemon status`

Den Zustand des Daemons und den nächsten Ablauf anzeigen

Verwendung: `obit daemon status [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu status |
| `--json` | keiner | `false` | JSON ausgeben |

### `obit daemon stop`

Den Daemon stoppen

Verwendung: `obit daemon stop [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu stop |

### `obit docker`

Docker-Container anhand ihrer unveränderlichen vollständigen ID und ihrer Erstellungszeit verfolgen. Beim Ablauf wird der Container gestoppt und seine beschreibbare Schicht bleibt erhalten. Beim Wiederbeleben wird er gestartet. Nach Ablauf von [graveyard] retention_days entfernt das endgültige Löschen gestoppte Container, ohne Volumes zu entfernen; das gilt auch im Papierkorbmodus. Images, Volumes und Netzwerke bleiben erhalten.

Verwendung: `obit docker [command]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu docker |

### `obit docker add`

Einen Docker-Container registrieren; beim Ablauf wird er gestoppt

Verwendung: `obit docker add <container name|ID> <lifespan> [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu add |
| `--json` | keiner | `false` | JSON ausgeben |

### `obit docker list`

Registrierte Docker-Container auflisten

Verwendung: `obit docker list [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu list |
| `--json` | keiner | `false` | JSON ausgeben |

### `obit doctor`

Installation, Integrationen und Daten prüfen; geschrieben wird nur der eigene Bericht

Verwendung: `obit doctor [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu doctor |
| `--json` | keiner | `false` | JSON ausgeben |
| `--open` | keiner | `false` | Den vollständigen HTML-Bericht im Standardbrowser öffnen |
| `--verbose` | keiner | `false` | Alle Prüfungen als vollständigen Text ausgeben, mit Details zu den Fähigkeiten der Agenten |

### `obit extend`

Verlängern: Abläufe nach hinten schieben (bei bereits abgelaufenen ab jetzt gerechnet)

Verwendung: `obit extend <id|path>... <duration> [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu extend |
| `--json` | keiner | `false` | JSON ausgeben |

Beispiele:

```text
obit extend 3 1d
obit extend debug.log 30m
```

### `obit git-clean`

Die vom Testlauf von git clean ausgewählten Pfade begraben

Verwendung: `obit git-clean [options] [--] [pathspec]... [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu git-clean |

### `obit graveyard`

Auflisten, was auf dem Friedhof liegt und noch wiederbelebt werden kann

Aliase: `graveyard`, `graves`
Verwendung: `obit graveyard [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu graveyard |
| `--here` | keiner | `false` | Nur Originalpfade anzeigen, die im aktuellen Verzeichnis und seinen Unterverzeichnissen liegen |
| `--json` | keiner | `false` | JSON ausgeben |

### `obit help`

Hilfe zu jedem Befehl der Anwendung anzeigen.

Verwendung: `obit help [command] [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu help |

### `obit hook`

Einstiegspunkte für Agent-Hooks (vom Agenten aufgerufen, nicht von Menschen)

Verwendung: `obit hook [command]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu hook |

### `obit hook antigravity`

Ein Hook-Ereignis von antigravity von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook antigravity [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu antigravity |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook audit`

Audit-Protokolle der Hooks und des Löschschutzes anzeigen

Verwendung: `obit hook audit [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu audit |
| `--lines` | `-n` | `40` | Anzahl der Zeilen pro Quelle |

### `obit hook claude`

Ein Hook-Ereignis von claude von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook claude [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu claude |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook cline`

Ein Hook-Ereignis von cline von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook cline [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu cline |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook codebuddy`

Ein Hook-Ereignis von codebuddy von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook codebuddy [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu codebuddy |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook codebuddy-ide`

Ein Hook-Ereignis von codebuddy-ide von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook codebuddy-ide [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu codebuddy-ide |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook codex`

Ein Hook-Ereignis von codex von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook codex [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu codex |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook copilot`

Ein Hook-Ereignis von copilot von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook copilot [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu copilot |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook cursor`

Ein Hook-Ereignis von cursor von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook cursor [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu cursor |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook deepseek-harness`

Ein Hook-Ereignis von deepseek-harness von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook deepseek-harness [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu deepseek-harness |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook droid`

Ein Hook-Ereignis von droid von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook droid [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu droid |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook gemini`

Ein Hook-Ereignis von gemini von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook gemini [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu gemini |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook hermes`

Ein Hook-Ereignis von hermes von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook hermes [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu hermes |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook kilocode`

Ein Hook-Ereignis von kilocode von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook kilocode [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu kilocode |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook kimi`

Ein Hook-Ereignis von kimi von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook kimi [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu kimi |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook lingma`

Ein Hook-Ereignis von lingma von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook lingma [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu lingma |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook minimax-code`

Ein Hook-Ereignis von minimax-code von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook minimax-code [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu minimax-code |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook omp`

Ein Hook-Ereignis von omp von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook omp [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu omp |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook openclaw`

Ein Hook-Ereignis von openclaw von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook openclaw [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu openclaw |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook opencode`

Ein Hook-Ereignis von opencode von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook opencode [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu opencode |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook pi`

Ein Hook-Ereignis von pi von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook pi [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu pi |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook qoder`

Ein Hook-Ereignis von qoder von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook qoder [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu qoder |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook qoder-cn`

Ein Hook-Ereignis von qoder-cn von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook qoder-cn [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu qoder-cn |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook trae`

Ein Hook-Ereignis von trae von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook trae [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu trae |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook trae-cn`

Ein Hook-Ereignis von trae-cn von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook trae-cn [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu trae-cn |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook vibe`

Ein Hook-Ereignis von vibe von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook vibe [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu vibe |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook zcode`

Ein Hook-Ereignis von zcode von stdin verarbeiten; bei Fehlern wird die Aktion zugelassen

Verwendung: `obit hook zcode [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--event` | keiner | `""` | Ereignisname des Hosts, falls er im Payload fehlt |
| `--help` | `-h` | `false` | Hilfe zu zcode |
| `--notice-only` | keiner | `false` | Hinweiskontext ohne den Löschschutz übermitteln |

### `obit hook finish`

Die von einer Sitzung hinterlassenen Dateien berechnen (von den Hooks im Hintergrund gestartet)

Verwendung: `obit hook finish [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu finish |
| `--project` | keiner | `""` | Stammverzeichnis des Projekts |
| `--session` | keiner | `""` | ID der Agent-Sitzung |

### `obit init`

Die Hooks und Anweisungen von Obituary für einen Agenten installieren (oder mit --uninstall entfernen)

Verwendung: `obit init [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--agent` | keiner | `[claude]` | Agenten, die eingebunden werden (Option wiederholen oder mit Komma trennen; Standard: claude) |
| `--context-menu` | keiner | `false` | Klassische Explorer-Kontextmenüs installieren (standardmäßig aktiviert) |
| `--dry-run` | keiner | `false` | Nur die Änderungen ausgeben |
| `--format` | keiner | `"text"` | Ausgabeformat für --show: text oder markdown (Agentenabdeckung) |
| `--global` | `-g` | `false` | Für den ganzen Benutzer statt nur für dieses Projekt |
| `--guard-rm` | keiner | `false` | Unterstützte Löschbefehle verweigern und wiederherstellbare obit-Alternativen vorschlagen |
| `--help` | `-h` | `false` | Hilfe zu init |
| `--mcp` | keiner | `false` | Den MCP-stdio-Server von Obituary registrieren |
| `--no-context-menu` | keiner | `false` | Explorer-Kontextmenüs überspringen |
| `--no-shortcuts` | keiner | `false` | Startmenü- und Desktopverknüpfungen überspringen |
| `--no-skill` | keiner | `false` | Den Obituary-Agent-Skill unverändert lassen |
| `--show` | keiner | `false` | Anzeigen, was installiert ist |
| `--skill` | keiner | `false` | Den Obituary-Agent-Skill installieren (standardmäßig aktiviert) |
| `--uninstall` | keiner | `false` | Entfernen, was Obituary hinzugefügt hat |
| `--verbose` | keiner | `false` | Mit --show: die Fähigkeiten jedes Agenten im Detail auflisten |
| `--yes` | keiner | `false` | Ohne Nachfrage anwenden |

Beispiele:

```text
obit init -g  # Claude Code, für alle Projekte (~/.claude)
obit init  # Claude Code, nur dieses Projekt (.claude/)
obit init -g --agent codex  # Codex (~/.codex/hooks.json + AGENTS.md)
obit init -g --uninstall  # Nur entfernen, was Obituary hinzugefügt hat
obit init -g --show  # Was installiert ist
obit init --show --format markdown > docs/agents.md
```

### `obit list`

Verfolgte Pfade und Docker-Container anzeigen, der früheste Ablauf zuerst

Aliase: `list`, `ls`
Verwendung: `obit list [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu list |
| `--json` | keiner | `false` | JSON ausgeben |

### `obit log`

Das Auditprotokoll von allem anzeigen, was Obituary getan hat

Verwendung: `obit log [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu log |
| `--json` | keiner | `false` | JSON ausgeben |
| `--lines` | `-n` | `20` | Anzahl der Ereignisse |

### `obit mcp`

Die Werkzeuge von Obituary über MCP stdio bereitstellen

Verwendung: `obit mcp [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu mcp |

### `obit mktemp`

Ein temporäres Verzeichnis anlegen, das sich selbst begräbt, und seinen Pfad ausgeben

Verwendung: `obit mktemp <lifespan> [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu mktemp |
| `--json` | keiner | `false` | JSON ausgeben |
| `--name` | keiner | `"tmp"` | Namenspräfix für das Verzeichnis |
| `--session` | keiner | `""` | Agent-Sitzung, zu der das Verzeichnis gehört |

Beispiele:

```text
dir=$(obit mktemp 1d)
obit mktemp 2h --name screenshots
```

### `obit proposals`

Registrierungen, die auf Genehmigung warten, und solche, die ohne Nachfrage erfolgt sind

Verwendung: `obit proposals [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu proposals |
| `--json` | keiner | `false` | JSON ausgeben |

### `obit purge`

Ausgewählte Datei-Begräbnisse oder solche mit abgelaufener Aufbewahrungsfrist löschen. --all und ausdrückliche Angaben (ID oder Pfad) überspringen die Aufbewahrungsfrist und fragen daher zuerst nach; außerhalb eines Terminals erfordern sie --yes. Bei Docker-Containern muss [graveyard] retention_days abgelaufen sein, auch mit --all oder einer ausdrücklichen ID; laufende Container gelten als wiederbelebt. Beim Entfernen von Docker-Containern bleiben Volumes erhalten, und es wird nie erzwungen. Im Papierkorbmodus werden Container genauso behandelt.

Aliase: `purge`, `cremate`
Verwendung: `obit purge [id|original path]... [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--all` | keiner | `false` | Alle Begräbnisse dauerhaft löschen, nicht nur die mit abgelaufener Aufbewahrungsfrist |
| `--dry-run` | keiner | `false` | Nur anzeigen, was passieren würde |
| `--help` | `-h` | `false` | Hilfe zu purge |
| `--json` | keiner | `false` | JSON ausgeben |
| `--yes` | keiner | `false` | --all oder die ausgewählten Begräbnisse ohne Nachfrage dauerhaft löschen (Agenten: zuerst den Benutzer fragen) |

### `obit reject`

Vorschläge ablehnen

Verwendung: `obit reject <proposal id>... [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu reject |
| `--json` | keiner | `false` | JSON ausgeben |

### `obit remove-item`

Dateisystemelemente mit einer begrenzten Teilmenge der PowerShell-Parameter begraben.

Verwendung: `obit remove-item [-Recurse] [-Force] [-Path|-LiteralPath] <path> [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu remove-item |

### `obit resurrect`

Dateien wiederherstellen, ohne etwas zu überschreiben; begrabene Docker-Container per ID starten

Aliase: `resurrect`, `raise`
Verwendung: `obit resurrect <id|original path>... | --last [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu resurrect |
| `--json` | keiner | `false` | JSON ausgeben |
| `--last` | keiner | `false` | Den letzten Löschstapel der aktuellen Sitzung oder des Git-Projekts wiederherstellen |

### `obit rm`

Pfade sofort auf den Friedhof verschieben (Wiederherstellung mit obit resurrect)

Verwendung: `obit rm [options] <path>... [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu rm |

### `obit rmdir`

Pfade sofort auf den Friedhof verschieben (Wiederherstellung mit obit resurrect)

Verwendung: `obit rmdir [options] <path>... [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu rmdir |

### `obit rules`

Die hier geltenden Regeln anzeigen: eingebaute Regeln plus die .obitrules des Projekts, sobald ihnen vertraut wird

Verwendung: `obit rules [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu rules |
| `--json` | keiner | `false` | JSON ausgeben |

### `obit rules apply`

Pfade per Regel registrieren (von Git verfolgte Dateien werden nie registriert)

Verwendung: `obit rules apply <path>... [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu apply |
| `--json` | keiner | `false` | JSON ausgeben |
| `--session` | keiner | `""` | Agent-Sitzung, aus der die Pfade stammen |

### `obit rules check`

Anzeigen, was die Regeln für Pfade registrieren würden (ändert nichts)

Verwendung: `obit rules check <path>... [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu check |
| `--json` | keiner | `false` | JSON ausgeben |
| `--session` | keiner | `""` | Agent-Sitzung, aus der die Pfade stammen |

### `obit stats`

Was Obituary getan hat: verfolgt, begraben, freigegebener Speicherplatz, Wiederbelebungen

Aliase: `stats`, `eulogy`
Verwendung: `obit stats [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu stats |
| `--json` | keiner | `false` | JSON ausgeben |

### `obit sweep`

Jetzt ernten: alles Fällige begraben, ohne auf den Hintergrundprozess zu warten

Aliase: `sweep`, `reap`
Verwendung: `obit sweep [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--dry-run` | keiner | `false` | Nur anzeigen, was passieren würde |
| `--help` | `-h` | `false` | Hilfe zu sweep |
| `--json` | keiner | `false` | JSON ausgeben |

### `obit trust`

Die .obitrules des Projekts prüfen und ihnen vertrauen (Änderungen erfordern eine neue Prüfung)

Verwendung: `obit trust [rules file] [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu trust |
| `--yes` | keiner | `false` | Ohne Nachfrage vertrauen (erst nach Prüfung der Regeln) |

### `obit uninstall`

Die Integrationen und Hintergrunddienste von Obituary entfernen; Daten bleiben standardmäßig erhalten

Verwendung: `obit uninstall [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--dry-run` | keiner | `false` | Nur den Deinstallationsplan ausgeben; keine Dateien oder Dienste ändern |
| `--help` | `-h` | `false` | Hilfe zu uninstall |
| `--purge-data` | keiner | `false` | Aufgelistete Gräber dauerhaft löschen und die Obituary gehörenden Daten entfernen; unbekannte Dateien behalten |
| `--yes` | keiner | `false` | Den Plan bestätigen (und das Löschen der Daten nur, wenn auch --purge-data angegeben ist) |

### `obit untrust`

Die .obitrules des Projekts nicht mehr anwenden

Verwendung: `obit untrust [rules file] [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Hilfe zu untrust |

### `obit update`

Ein stabiles Update prüfen oder installieren

Verwendung: `obit update [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--apply` | keiner | `false` | Das zwischengespeicherte neuere stabile Release herunterladen, prüfen und installieren (eine manuelle Installation ersetzt nur die eigenen Programme von Obituary) |
| `--check` | keiner | `false` | GitHub jetzt prüfen, auch wenn automatische Prüfungen deaktiviert sind |
| `--help` | `-h` | `false` | Hilfe zu update |
| `--relaunch-gui` | keiner | `false` | Das kleine Fenster nach dem Update wieder öffnen (Windows) |
| `--wait-pid` | keiner | `0` | Vor dem Update warten, bis dieser Prozess beendet ist (Windows) |

### `obit watch`

Zur Einzelbestätigung unter `[watch]` die Einstellung `file_expiry = "propose"` setzen. Mit `obit proposals` prüfen und mit `obit approve <ID>` bestätigen. Abgelaufene Unterordner erzeugen weiterhin Vorschläge und bleiben unter macOS und Linux auch nach Bestätigung am alten Ort.

Jedem Element auf oberster Ebene eines Ordners eine eigene Lebensdauer geben

Verwendung: `obit watch <directory> <lifespan> | watch --list | watch --remove <directory> [flags]`

| Parameter | Kurz | Standardwert | Beschreibung |
| --- | --- | --- | --- |
| `--age` | keiner | `"first-seen"` | Altersbasis: first-seen oder modified (erfordert Genehmigung) |
| `--clear-exclusion` | keiner | `0` | Diese Ausschluss-ID ausdrücklich aufheben (mit --policy) |
| `--help` | `-h` | `false` | Hilfe zu watch |
| `--json` | keiner | `false` | JSON ausgeben |
| `--list` | keiner | `false` | Ordnerrichtlinien auflisten |
| `--no-daemon` | keiner | `false` | Den Hintergrundprozess nicht starten |
| `--policy` | keiner | `0` | ID der Richtlinie, deren Ausschluss aufgehoben werden soll |
| `--remove` | keiner | `""` | Eine Richtlinie entfernen und ihre registrierten Lebensdauern abbrechen |
| `--remove-id` | keiner | `0` | Eine Ordnerrichtlinie per ID entfernen, auch bei fehlenden Pfaden |

<!-- generated-command-reference:end -->
