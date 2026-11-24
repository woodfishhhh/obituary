# Konfigurationsreferenz

Obituary speichert seine Einstellungen in `config.toml` im Konfigurationsverzeichnis. Die Datei ist optional: Ohne sie, oder mit nur wenigen Zeilen darin, gilt für alles andere der Standardwert. `obit config` zeigt dir, wo die Datei liegt, ob es sie gibt und welchen Wert jede Einstellung gerade tatsächlich hat. `obit config --create` schreibt eine kommentierte Datei mit allen Standardwerten, was der einfachste Einstieg zum Bearbeiten ist.

Das Konfigurationsverzeichnis ist unter Windows `%APPDATA%\obituary`, unter macOS `~/Library/Application Support/obituary` und unter Linux `~/.config/obituary` (oder `obituary` unterhalb von `XDG_CONFIG_HOME`, falls gesetzt). Soll es woanders liegen, setze die Umgebungsvariable `OBIT_CONFIG_DIR`.

| Schlüssel | Standardwert | Zulässige Werte |
| --- | --- | --- |
| `graveyard.mode` | `graveyard` | `graveyard`, `recycle` |
| `graveyard.retention_days` | `7` | eine ganze Zahl von 1 bis 3650 |
| `proposals.mode` | `off` | `off`, `tiered`, `auto`, `ask` |
| `hooks.session_cleanup` | `true` | `true`, `false` |
| `hooks.session_summary` | `false` | `true`, `false` |
| `watch.stable_window` | `10m` | eine positive Dauer, etwa `30s`, `10m`, `1h30m` |
| `watch.file_expiry` (macOS/Linux) | `quiet` | `quiet`, `propose` |
| `explorer.enabled` | `true` | `true`, `false` |
| `explorer.everywhere` | `false` | `true`, `false` |
| `explorer.in_git` | `false` | `true`, `false` |
| `ui.language` | `auto` | `auto`, `en`, `zh-CN` |
| `ui.tray` | `true` | `true`, `false` |
| `ui.taskbar` | `true` | `true`, `false` |
| `update.check` | `true` | `true`, `false` |
| `telemetry.enabled` | `true` | `true`, `false` |
| `agent_shells` | nicht gesetzt | eine Tabelle: Agentenname = `bash` oder `powershell` |
| `codex_windows_shell` | nicht gesetzt | `bash`, `powershell` |

## Wohin Abgelaufenes kommt

Standardmäßig landet etwas, dessen Lebensdauer abgelaufen ist, auf dem Friedhof von Obituary. `graveyard.retention_days` Tage lang (7, wenn du nichts änderst) kannst du es mit `obit resurrect` an seinen Platz zurückholen; erst danach wird es endgültig gelöscht. Die Zahl kann zwischen 1 und 3650 liegen.

Unter Windows und Linux aktiviert `graveyard.mode = "recycle"` den Systempapierkorb. Stelle die Dateien dort wieder her, nicht mit `obit resurrect`. Unter macOS gibt es nur den Friedhof.

## Automatische Registrierung und Agentensitzungen

`proposals.mode` entscheidet, ob Obituary Dateien von selbst eine Lebensdauer gibt. Der Standardwert `off` tut nichts automatisch: Es kommt nur hinein, was ein Agent oder du über Befehle, das Kontextmenü oder Ordnerrichtlinien registrierst. Mit `tiered` werden Dateien, die der Agent in dieser Sitzung angelegt hat, und Container, die die Sitzung gestartet hat, sofort aufgenommen und du erfährst es hinterher; mit `obit spare` nimmst du eine Datei wieder heraus. Alles andere wartet in `obit proposals` auf deine Zustimmung. `auto` registriert alles, was die automatische Registrierung findet, und sagt es dir hinterher. `ask` fragt bei jedem Eintrag zuerst nach. In jedem Modus werden Dateien, die Git bereits verfolgt, sowie Abhängigkeiten, Build-Ausgaben und Caches (`node_modules`, `dist`, `__pycache__` und Ähnliches) nie automatisch registriert. Die Regeln stehen unter [Welche Dateien registriert werden](./auto-register).

`hooks.session_cleanup` ist standardmäßig an. Gegen Ende jeder Sitzung erfährt der Agent, welche Dateien er angelegt hat, die keine Lebensdauer haben und die Git nicht verfolgt, und er wird gebeten, seinen Zwischendateien eine Lebensdauer zu geben (`obit mark`) oder sie zu begraben; Ergebnisse bleiben. `hooks.session_summary` ist standardmäßig aus; schaltest du es ein, erfährt der Agent zu Beginn einer Sitzung außerdem, welche Einträge im Projekt innerhalb von 24 Stunden ablaufen und was kürzlich begraben wurde. Es ist aus, um Tokens zu sparen, denn der Agent wird ohnehin informiert, sobald er einen begrabenen Pfad berührt.

## Ordnerrichtlinien und Datumsangaben im Explorer

`watch.stable_window` ist die Geduld einer Ordnerrichtlinie: Eine neue Datei muss so lange unverändert bleiben, bevor sie verschoben wird, damit nichts mitgenommen wird, was noch heruntergeladen wird. Es gilt das Zeitformat von Go, eine Zahl gefolgt von `s`, `m` oder `h`, die sich wie in `1h30m` aneinanderreihen lassen, und der Wert muss größer als null sein.

Die Einstellungen `explorer.*` sind nur unter Windows von Belang. Obituary lässt den Explorer neben dem Dateinamen das Begräbnisdatum anzeigen, während der eigentliche Name unverändert bleibt. Standardmäßig geschieht das in Downloads, auf dem Desktop (einschließlich Unterordnern) und an Orten, die eine Ordnerrichtlinie verwaltet. Mit `explorer.everywhere` zeigen es auch andere verfolgte Orte an; Git-Arbeitsbereiche bleiben ausgenommen, solange du `explorer.in_git` nicht auf `true` setzt; und `explorer.enabled = false` schaltet es ganz ab. Siehe [Ordnerrichtlinien](./folder-policies).

## Oberfläche

`ui.language` legt die Sprache der Befehlszeile, des Fensters, des Infobereichs-Symbols und des Kontextmenüs fest. Bisher gibt es nur `en` und `zh-CN`. `auto` sieht zuerst in die Umgebungsvariable `OBIT_LANG`, dann in `LC_ALL`, `LC_MESSAGES` und `LANG` und zuletzt auf die Oberflächensprache des Systems. `ui.tray` blendet das Symbol im Infobereich (unter macOS das Symbol in der Menüleiste) ein oder aus; ist es ausgeblendet, laufen die Bereinigung im Hintergrund und die Update-Prüfungen weiter. `ui.taskbar` gilt nur für Windows und bestimmt, ob das Fenster, solange es offen ist, eine Schaltfläche in der Taskleiste bekommt.

## Update-Prüfung und Nutzungsstatistik

Ist `update.check` an, liest Obituary die öffentlichen Release-Informationen auf GitHub höchstens einmal am Tag und sendet keine Nutzungsdaten. Ist es aus, kannst du jederzeit selbst `obit update --check` ausführen. `telemetry.enabled` steuert die anonyme Nutzungsstatistik. Es ist ein eigener Schalter, unabhängig von den Update-Prüfungen: `obit config telemetry off` schaltet nur die Statistik ab und löscht von diesem Computer die Installations-ID und jeden noch ausstehenden Bericht. Die Einzelheiten stehen in der [Datenschutzerklärung](./privacy).

## Welche Shell ein Agent unter Windows nutzt

Der Löschschutz muss den Löschbefehl lesen, den ein Agent gleich ausführen will, um den gleichwertigen `obit`-Befehl vorschlagen zu können. Unter Windows kann ein Agent bash oder PowerShell verwenden, und Obituary kann nicht erkennen, welche, also erklärst du es: Dieser Agent läuft auf diesem Windows-Rechner in dieser Shell. Ohne Erklärung passieren Befehle mit mehrdeutiger Bedeutung ungehindert.

```toml
[agent_shells]
codex = "powershell"
```

Die Schlüssel von `agent_shells` sind Agentennamen. Derzeit lesen `codex`, `codebuddy`, `codebuddy-ide` und `minimax-code` die Tabelle. `codex_windows_shell` ist die ältere Schreibweise, nur für Codex; sind sie und `agent_shells.codex` mit verschiedenen Werten gesetzt, wird die Konfiguration abgelehnt.

## Wie sich ein Fehler zeigt

Obituary ignoriert einen unbekannten Schlüssel nicht stillschweigend. Täte es das, bliebe bei `retention = 30` die Aufbewahrung von 7 Tagen in Kraft, während du glaubst, sie sei auf 30 gestiegen. Deshalb meldet `obit config` bei allem, was es nicht erkennt, den Dateipfad, die Zeilennummer und den vollständigen Schlüsselnamen und schlägt den richtigen Schlüssel vor, wenn die Schreibweise nah dran ist:

```text
obit: config file has errors: …\config.toml:2: unknown key "graveyard.retention" (did you mean "graveyard.retention_days"?)
```

Ein Wert außerhalb des Bereichs wird genauso gemeldet, mit der Einstellung und dem, was sie zulässt, zum Beispiel `graveyard.retention_days must be between 1 and 3650`. Auch ein TOML-Syntaxfehler lässt `obit config` scheitern. Korrigiere die Datei und führe den Befehl erneut aus; wenn er nichts mehr beanstandet, ist es behoben.

## Häufige Befehle

```powershell
obit config                  # Dateispeicherort, Datenverzeichnis und die geltenden Werte
obit config --create         # schreibt die kommentierte Standarddatei
obit config --language en    # speichert die Oberflächensprache: auto, en oder zh-CN
obit config telemetry off    # schaltet die Nutzungsstatistik aus (on schaltet sie wieder ein)
```

`--create` überschreibt nie eine vorhandene Datei; gibt es die Datei schon, meldet der Befehl einen Fehler und bricht ab. `--language` legt die Datei mit den Standardwerten an, falls es sie noch nicht gibt.
