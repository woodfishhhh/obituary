# Fenster, Infobereich und Kontextmenü

Dateien wiederbeleben, Vorschläge genehmigen und Einstellungen ändern geht auch ohne Kommandozeile.

## Das Symbol im Infobereich

`obit init` richtet ein Symbol im Infobereich ein, das bei der Anmeldung erscheint. Sein Menü bietet:

- die zuletzt begrabenen Einträge; klicke einen an, um ihn wiederzubeleben;
- Einträge, die innerhalb der nächsten 24 Stunden ablaufen;
- die automatische Bereinigung anhalten oder fortsetzen;
- „Open Obituary“ zum Öffnen des kleinen Fensters; fehlt `obit-gui.exe` im Verzeichnis von `obitd.exe`, erklärt ein Hinweis das Problem;
- Beenden (die geplante Aufgabe läuft weiter).

## Das kleine Fenster

`obit-gui.exe` hat fünf Seiten: Upcoming, Graveyard, Decisions, Settings und Logs. Du öffnest das Fenster über „Obituary“ im Startmenü, die Desktop-Verknüpfung oder „Open Obituary“ im Tray-Menü.

- Die obere Leiste der Startseite zeigt „Auto registration: off“ oder „on“. Ein Klick öffnet die Einstellungen. Oben auf Decisions und unter Settings kannst du zwischen aus (Standard: nur du und der Agent registrieren), abgestuft, vollautomatisch und immer nachfragen wählen; siehe [Was registriert wird](./auto-register). Weiter unten stehen unter „Handled automatically“ die ohne Rückfrage registrierten Einträge. Je Eintrag kannst du die Registrierung mit einem Klick rückgängig machen (die Frist endet, der Eintrag bleibt an Ort und Stelle) oder die Lebensdauer verlängern.
- Fällige Einträge, die nicht begraben werden konnten, etwa weil ein Programm die Datei offen hält, erscheinen unter „Burials that failed“ auf Decisions. Dort stehen Grund und nächster Versuch. Die Wartezeit zwischen den Versuchen wächst. Nach wiederholten Fehlern wartet Obituary auf dich: „Retry“ versucht es sofort erneut, „Stop tracking“ lässt den Eintrag an seinem Platz und beendet die Überwachung.
- Läuft Docker nicht, ist „Approve“ bei Containervorschlägen nicht verfügbar. Die Karte nennt den Grund. Sobald Docker läuft, kannst du den Vorschlag genehmigen.
- Wenn du einer `.obitrules` vertraust, gilt das nur für die im Fenster angezeigte Version.
- Fenster, Infobereich und Kontextmenü verwenden `[ui] language` aus `config.toml` (`auto`, `zh-CN` oder `en`; `auto` übernimmt die Windows-Sprache). Du kannst die Sprache auch in den Einstellungen ändern.
- Das kleine Fenster gibt es nur für x64. Es benötigt die WebView2-Laufzeit, die zu Windows 11 gehört und auf den meisten Windows-10-Rechnern vorhanden ist.

### Updates

Unter Settings → About and updates kannst du eine neuere stabile Version installieren. Obituary schließt Fenster und laufendes Tray, aktualisiert über den ursprünglichen Installationsweg (Scoop, WinGet oder Skript) und versucht anschließend, Tray und Fenster wieder zu öffnen. Fehler und Protokollpfade stehen auf derselben Seite. Bei einer manuell entpackten Installation lade den Ersatz von der Release-Seite herunter.

### Acht Ansichten

Unter Settings → Appearance wechselst du die Ansicht. Das wirkt sofort und nur auf das Fenster dieses Computers.

| Ansicht | Beschreibung |
|---|---|
| Kalender | Eine Spalte pro Tag; jede Datei steht am Tag des Begrabens oder der dauerhaften Löschung. Ziehe sie auf einen späteren Tag, um die Lebensdauer zu verlängern. |
| Eine Zeile | Eine Eingabezeile, alles per Tastatur: Datei suchen und mit Enter wiederbeleben oder verlängern. |
| Tunnel | Ein 3D-Zeittunnel: Je näher eine Datei steht, desto früher läuft sie ab. Scrolle durch die Zeit. |
| Nachtwache | Dunkle Ansicht, eine Lampe pro Datei. Je schwächer das Licht, desto näher der Ablauf. |
| Anzeigen | Nachrufe wie in einer Zeitung, in Spalten mit Serifenschrift. Beim Wiederbeleben erscheint ein Stempel. |
| Horizont | Die registrierten Dateien liegen über der Erde, der Friedhof darunter. |
| Mica | Windows-11-Ansicht mit Seitenleiste. |
| Nacht | Ruhige, dunkle Ansicht mit einer Leiste am oberen Rand. |

## Kontextmenü

`obit init` installiert das Explorer-Kontextmenü standardmäßig nur für das aktuelle Benutzerkonto. Klicke mit der rechten Maustaste auf eine Datei oder einen Ordner und wähle „Add to Obituary“. Du hast folgende Möglichkeiten:

- **1 Tag / 7 Tage / 30 Tage**: Der Eintrag wird nach Ablauf begraben. Bis dahin nutzt du ihn wie gewohnt.
- **Bury now (recoverable)**: Der Eintrag kommt sofort auf den Friedhof. Während der Aufbewahrungsfrist (standardmäßig sieben Tage) kannst du ihn wiederbeleben.

Du kannst mehrere Dateien gleichzeitig auswählen und hinzufügen. Ein Hinweis nennt die Anzahl der hinzugefügten Einträge, den Zeitpunkt des Begrabens und die Zahl der Fehler. Im neuen Kontextmenü von Windows 11 fehlen die Einträge: Wähle „Weitere Optionen anzeigen“ oder halte beim Rechtsklick die Umschalttaste gedrückt. `obit init --dry-run` listet die Menüeinträge mit den zugehörigen Befehlen auf. `obit doctor` prüft die Installation.

Unter macOS und Linux gibt es noch kein Kontextmenü. Nutze `obit mark <path> 7d` zum Registrieren oder `obit bury <path>` zum sofortigen Begraben.

## Symbole und Menüs

`obit init` richtet die folgenden Elemente standardmäßig ein. Unter Settings → Icons and menus lassen sie sich einzeln schalten:

| Einstellung | Wirkung |
|---|---|
| Tray-Symbol | Zeigt Obituary im Infobereich. Die Bereinigung läuft auch bei ausgeblendetem Symbol. |
| Taskleistensymbol | Bestimmt, ob das geöffnete Fenster einen Platz in der Taskleiste belegt. |
| Startmenü | „Obituary“ im Startmenü. |
| Desktop-Verknüpfung | "Obituary" auf dem Desktop |
| Kontextmenü | Das Menü „Add to Obituary“ im Explorer. |

Mit `obit init --no-context-menu` lässt du das Kontextmenü bei der Installation aus, mit `--no-shortcuts` die Verknüpfungen im Startmenü und auf dem Desktop.

Wie der Explorer die Begräbnisdaten anzeigt, steht unter [Ordnerrichtlinien](./folder-policies#dates-in-explorer).
