# Löschschutz

Einen Benutzerordner oder ein ganzes Laufwerk zu löschen ist keine hypothetische Gefahr. Ist der Schutz aktiv und führt der Agent `rm -rf build`, `rmdir`, `git clean -fdx`, `Remove-Item -Recurse` oder eine ähnliche Löschung aus, stoppt Obituary die Ausführung und nennt einen Ersatzbefehl wie `obit rm -rf build`. Nach dessen Verwendung landen die Einträge auf dem Friedhof statt zu verschwinden; `obit resurrect <id>` stellt sie wieder her.

```powershell
obit init -g --guard-rm                 # Claude Code; add --agent codex for Codex. Off by default
```

Der Löschschutz erkennt `docker volume rm`, `docker volume remove`, `docker volume prune`, `docker compose down -v` (oder `--volumes`) und `docker system prune --volumes`. Nur bei einem einfachen `docker volume rm` mit ausdrücklichen Namen schlägt er `obit bury --docker-volume` vor.

Der Löschschutz stoppt außerdem `wsl --unregister` (einschließlich `wsl.exe`, vollständiger Pfade, `=`-Schreibweisen sowie Aufrufen über `cmd /c` oder `powershell -Command`), da dadurch die gesamte WSL-Distribution samt virtueller Festplatte (`ext4.vhdx`) dauerhaft zerstört wird. Obituary begräbt und restauriert keine WSL-Distributionen; der Ablehnungshinweis verweist vor einer manuellen Bestätigung auf `wsl --export <distro> <backup.tar>` als Sicherungsschritt. Lesende oder nicht destruktive Befehle wie `wsl --list`, `wsl --status`, `wsl --export`, `wsl --terminate` und `wsl --shutdown` laufen weiter.

## Ein echter Lauf

Bei aktivem Löschschutz erhielt der Hook einen Claude-Code-Bash-Aufruf, `rm -rf build`, und antwortete wie folgt; `rm` wurde nie ausgeführt:

```json
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Obituary's delete guard is on: `'C:/work/obit.exe' rm -rf build` provides restorable deletion; `obit resurrect` restores graveyard entries."}}
```

Der Agent wechselt zu Obituary. Der Ordner wird begraben und mit einem Befehl wiederhergestellt.

```console
$ obit rm -rf build
$ obit graveyard --here
ID  BURIED            PURGE  SIZE  PATH
#2  2026-11-11 00:36  in 7d  23 B  C:\work\build
$ obit resurrect --last
#2 resurrected  C:\work\build
```

## Grundsätze

- **Ablehnen, nicht umschreiben.** Obituary schreibt den Agentenbefehl nicht um. Claude Code prüft Berechtigungen gegen den umgeschriebenen Befehl, sodass eine `deny`-Regel für das Original unwirksam werden könnte. Durch Ablehnen des Originals und erneutes Ausgeben des Ersatzbefehls bleiben alle Berechtigungsregeln erhalten.
- **Jede wörtliche Löschung wird gestoppt.** `rm`, `rmdir`, `git clean` und `Remove-Item` (einschließlich `del`, `rd`, `ri`) werden an jeder Befehlsposition erkannt: nach `&&`, `||` oder `;`, in Pipelines, `$(…)`, Subshells, Bedingungen und Schleifen. Das gilt etwa für `rm -rf build && npm run build`, `rm -rf build 2>/dev/null` und `git clean -fdx && npm ci`. `rm` in Zeichenketten, Kommentaren oder `echo`-Argumenten zählt nicht; ebenso wenig `rm --help`, `git clean -n` und `Remove-Item -WhatIf`.
- **Zur Laufzeit gewählte Ziele zuerst auflisten.** `find -delete`, `find -exec rm` (auch `-execdir`), `xargs rm` und PowerShells `Get-ChildItem … | Remove-Item` werden ohne Umschreibung abgelehnt. Liste und prüfe die Treffer, dann übergib die expliziten Pfade einzeln oder gesammelt an `obit rm -- <path>...`. Ein nur suchendes oder auflistendes `find` läuft weiter. Beliebige Wrapper wie `sudo rm` oder `bash -c '…'` werden nicht untersucht.
- **Nie auf echtes Löschen zurückfallen.** Jeder an `obit rm`, `obit remove-item` oder `obit git-clean` übergebene Pfad wird begraben. Bei einer Ablehnung folgt der Fehler dem Format von `rm` und der Exit-Code ist ungleich null.
- **Der echte Pfad wird geprüft.** Zwischen beabsichtigtem und ausgeführtem Befehl liegen Shell-Zitate, Escapes und manchmal ein Proxy. Ein verlorenes Zeichen kann `rm -rf "$OUT"/` zu `rm -rf /` machen. Vor dem Zugriff prüft `obit rm` den tatsächlichen Pfad auf dem Datenträger.
- **Unter Windows zählt Codex Ersetzung auch in Anführungszeichen.** Eine weitere Shell kann Text ausführen, den Bash wörtlich behandelt hätte. `$(…)` wird deshalb nach Bash und PowerShell geprüft, auch in Anführungszeichen, zitierten Heredocs und Kommentaren. Eine Löschung stoppt die Ausführung; Text mit Backticks kann in eine Datei geschrieben und als Dateipfad übergeben werden.
- Git-verfolgte Dateien werden ebenfalls begraben, damit nicht committete Änderungen wiederherstellbar bleiben. Das ist eine bewusste Agentenlöschung und nicht die automatische Registrierung.
- `obit rm -rf` auf einem Ordner mit `.git` begräbt das gesamte Repository. Echtes `rm -rf` würde es zerstören, Begraben lässt sich jedoch rückgängig machen. Der Schutz von System- und Benutzerordnern bleibt bestehen.
- **Der Ersatzbefehl ist bewusst eng.** Er darf nie mehr als das Original auswählen. Deshalb übersetzt Obituary nicht zwischen Shells und bietet nur für die Formen in der Tabelle einen Ersatz. Andernfalls wird die Löschung gestoppt und auf `obit bury <path>` sowie `obit git-clean` als wiederherstellbare Alternativen verwiesen.

## Was gestoppt wird und was passiert

| Host | Stopp mit Ersatzbefehl | Stopp mit Grund und allgemeiner Anleitung | Durchlauf |
|---|---|---|---|
| Claude Code (Bash-Werkzeug) | `rm`, `rmdir`, `git clean`, optional mit literaler `cd x` davor, verbunden durch `&&`, `;` oder Zeilenumbruch; `rm -f *.log` ohne Dateien, die mit `-` beginnen, `rm -rf ../dist`, `rm -- "$target"` mit Variable nach `--` | Jede andere wörtliche Löschung: weitere Anweisungen (`npm …`, `trap`, `hash`), Variablen vor `--`, POSIX-Pfade mit `/` am Anfang, Umleitungen, `\|\|`, Pipelines, `$(…)`, interaktive Löschungen, `/usr/bin/rm` mit Pfad | Befehle ohne Löschung |
| Codex unter WSL `/mnt/<drive>` | Ein einzelnes `rm` oder `rmdir` mit relativen Pfaden ohne `..`, `~`, Wildcards oder Anführungszeichen | Jede andere wörtliche Löschung, einschließlich `git clean` und Pfaden in das eigene WSL-Dateisystem | Arbeitsverzeichnis im eigenen WSL-Dateisystem |
| Codex mit deklarierter Windows PowerShell | Ein einzelnes `Remove-Item` oder Alias mit einem literalen Pfad (`-Recurse` und `-Force` erlaubt) | Jede andere wörtliche Löschung: mehrere Anweisungen, Pipelines, Parameter wie `-ErrorAction`, Providerpfade, `~`, Wildcards oder Backticks | Text in geschweiften Anführungszeichen |
| Codex unter Windows mit unbekannter Shell | — | Löschung in rohen Backtick- oder `$(…)`-Fragmenten | Andere Befehle |
| pwsh unter Linux, `git rm` | — | — | Alles läuft durch |

Für einen Ersatz muss die Löschung das **letzte Segment** des Befehls sein; davor sind nur Verbindungen mit `&&`, `;` oder Zeilenumbrüchen erlaubt. Für Namen mit abschließendem Punkt oder Leerzeichen gibt es keinen Ersatz, weil Windows sie normalisiert. Die wörtliche Löschung wird trotzdem gestoppt.

## Verwenden Sie es selbst

`obit rm`, `obit rmdir` und `obit git-clean` akzeptieren die üblichen nicht interaktiven Optionen von `rm`, `rmdir` und `git clean`. Nicht unterstützte Optionen wie `-i` oder `rmdir -p` erzeugen einen Fehler. `obit remove-item` ist eine einfache Variante von `Remove-Item` und akzeptiert nur einen literalen Pfad; für Wildcards verwende in Bash `obit rm *.log`.

Mit `OBIT_GUARD_RM=0` lässt sich der Schutz vorübergehend abschalten. `obit stats` zeigt die Zahl gestoppter Löschungen und die begrabene Datenmenge.
