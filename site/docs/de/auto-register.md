# Welche Dateien registriert werden

## Der Agent und du entscheiden; Obituary begräbt pünktlich

Der Agent und du entscheiden, was in Obituary gelangt (standardmäßig `[proposals] mode = "off"`):

- **Der Agent registriert seine eigenen Ergebnisse.** Für Debug-Skripte, Logs und Screenshots setzt er mit `obit mark <path> 2d` eine Lebensdauer, erstellt mit `obit mktemp 1d` temporären Platz und begräbt mit `obit bury`, was er nicht sicher behalten muss. `obit init` erklärt dem Agenten diese Regeln.
- **Du fügst über das Kontextmenü hinzu.** Klicke im Explorer, Finder, Nautilus oder Dolphin mit der rechten Maustaste auf Dateien oder Ordner, wähle „Add to Obituary“ und dann 1, 7 oder 30 Tage oder „Bury now (recoverable)“. Siehe [Fenster, Tray und Kontextmenü](./desktop).
- **Ordnerrichtlinien** geben Ordnern wie Downloads eine Lebensdauer. Siehe [Ordnerrichtlinien](./folder-policies).

Registrierte Einträge werden bei Ablauf begraben und können sieben Tage lang unverändert wiederbelebt werden. Nichts anderes wird automatisch aufgenommen.

- **Aufräumen am Sitzungsende.** Wenn ein Claude-Code- oder Codex-Agent stoppt, listet Obituary die in dieser Sitzung erstellten, noch nicht eingeordneten Dateien auf. Der Agent soll Debug-Skripten, Logs und Screenshots eine Lebensdauer geben oder sie begraben, während die Ergebnisse bleiben. Höchstens einmal pro Sitzung; ohne Einträge bleibt es still. Andere Agenten erhalten den Hinweis beim nächsten Sitzungsstart. `[hooks] session_cleanup = false` schaltet ihn aus.
- **Beim Upgrade von einer älteren Version** wird diese Einstellung einmalig ausgeschaltet. Zuvor automatisch registrierte Abhängigkeiten, Build-Ausgaben und Caches bleiben am Ort und erhalten keine Lebensdauer.

## Automatische Registrierung aktivieren, wenn Sie es wünschen

Der Hintergrundprozess verarbeitet automatische Registrierungen nach dem Schreiben. Daher kann es einen Moment dauern, bis eine neue Datei in `obit list` erscheint.

Aktiviere die Option unter Settings oder For you oder setze `[proposals] mode` in `config.toml`.

- `"tiered"` registriert Dateien, die der Agent in dieser Sitzung erzeugt, und [Docker-Container](./docker), die für das Projekt gestartet wurden, direkt. Eine Datei muss vom Hook beim Schreiben beobachtet worden sein, im Anfangssnapshot fehlen und eine spätere Erstellungszeit haben. Danach wird der Agent informiert; die Einträge erscheinen in `obit proposals` und unter „Handled automatically“ auf For you. Bereits vorhandene oder nicht beobachtete Dateien werden nur vorgeschlagen. `obit approve <id>` registriert vorgeschlagene Einträge; danach werden passende Regeln dieses Projekts direkt angewendet. `obit reject <id>` lehnt die automatische Registrierung für dieses Projekt ab.
- `"auto"` registriert alles, was die automatische Registrierung findet, direkt und informiert dich anschließend.
- `"ask"` hält alles zunächst als Vorschlag zurück.

```powershell
obit proposals           # proposals waiting for you, and what was registered without asking
obit approve 3           # approve: register what it lists
obit reject 3            # reject
obit spare 12            # undo an automatic registration: it stays where it is, without a lifespan
obit extend 12 1w        # keep it longer
```

- **Abhängigkeiten, Build-Ausgaben und Caches** (`node_modules`, `dist`, `.turbo`, `__pycache__` und Ähnliches) werden nie automatisch registriert oder vorgeschlagen. Für die Bereinigung musst du sie selbst mit `obit mark` registrieren.
- **Große Einträge**, die der Agent nicht erstellt hat, werden bei mehr als 1 GB oder 200 Dateien ebenfalls vorgeschlagen.
- **Sitzungsdifferenz.** Zu Beginn wird ein Snapshot im Datenverzeichnis erstellt und am Ende verglichen; das Repository bleibt unverändert. Regelübereinstimmungen folgen dem Modus, neue Dateien ohne Regel werden weder registriert noch vorgeschlagen.
- **Nach einem Moduswechsel** ordnet der Hintergrundprozess wartende Vorschläge neu ein.
- **Vor dem Begräbnis wird Git erneut gefragt.** Ein per Regel oder Vorschlag registrierter Eintrag wird nicht begraben, wenn er bei Ablauf bereits committet ist.
- **Vorschläge laufen selbst ab.** Committete oder gelöschte Einträge verschwinden; leere Vorschläge werden zurückgezogen, im Auditprotokoll vermerkt und nicht als Ablehnung gezählt.

Volumes werden nie automatisch registriert. Obituary verarbeitet nur ausdrücklich registrierte oder begrabene Volumes und löscht weder Images noch Netzwerke.

## Regeln

```powershell
obit rules                       # the rules in effect here (built-in plus a trusted .obitrules)
obit rules check debug.log out/  # preview what would be registered and by which rule (changes nothing)
obit rules apply debug.log       # register by rule
```

`obit rules apply` registriert jederzeit nach Regeln; bei aktivierter automatischer Registrierung nutzen auch die Hooks diese Regeln. Eingebaute Regeln decken typische Agentenreste wie `.playwright-mcp/`, `test-results/` und `*.log` ab. Die Sicherungen `*.bak`, `*.bak.*`, `*.orig`, `*.rej`, `*.old`, `*.backup` und `*~` bleiben sieben Tage erhalten; LevelDBs `LOG.old` bleibt unberührt. `.obitrules` im Projektstamm verwendet die Syntax von `.gitignore` und erhält hinter jedem Muster eine Lebensdauer.

```
output/**        7d      # every item under output gets its own lifespan
!output/keep/**          # never touch (negation wins, even over registering a parent folder)
test-results/    off     # turn off a built-in rule
```

Anders als bei `.gitignore` haben Ausschlüsse Vorrang vor der letzten Übereinstimmung. Unter Windows wird die Groß-/Kleinschreibung ignoriert. Ein ungültiges Muster wie ein nicht geschlossenes `[` nennt Datei und Zeile und pausiert die automatische Registrierung des Projekts.

Die Einheiten sind `s`, `m`, `h`, `d` und `w`; sie lassen sich etwa mit `1d12h` kombinieren.

## Harte Regeln für automatische Registrierung

- Nur unversionierte Einträge in einem Git-Arbeitsbaum werden registriert.
- Ein Ordner mit auch nur einer verfolgten Datei wird nie angefasst.
- Kann Git nicht antworten, wird übersprungen.
Eine vorhandene Lebensdauer wird nie verkürzt.
- Ein abgelehntes Projekt wird in keinem Modus direkt registriert. Ordnerrichtlinien und das Vertrauen in `.obitrules` warten immer auf deine Entscheidung.

## Projektregeln müssen zuerst vertrauenswürdig sein

Das `.obitrules` eines Repositorys bleibt unwirksam, bis du es geprüft und `obit trust` ausgeführt hast. So kann ein fremdes Repository keine privaten Dateien über Regeln an sich nehmen. Der SHA-256-Wert wird gespeichert; jede Änderung erfordert eine neue Prüfung. Nichtinteraktive Sitzungen brauchen `--yes`, in CI kannst du `OBIT_TRUST_PROJECT_RULES=1` setzen.

## Ein temporärer Ordner, der sich selbst vergräbt

```powershell
$dir = obit mktemp 1d            # .obit/tmp/ in a project (not in Git), otherwise the system temp folder
```

In bash: `dir=$(obit mktemp 1d)`.
