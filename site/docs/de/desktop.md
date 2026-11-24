# Fenster, Infobereich und Kontextmenü

Dateien wiederbeleben, Vorschläge genehmigen und Einstellungen ändern geht auch ohne Kommandozeile.

## Was jede der drei Plattformen bietet

Fenster, Infobereichs-Symbol und Desktop-Benachrichtigungen gibt es unter Windows, macOS und Ubuntu 24.04. Die Desktop-Ausgaben für macOS und Ubuntu sind vorerst Vorabversionen. Andere Linux-Distributionen haben keine Desktop-Ausgabe und nutzen die Befehlszeile.

| | Windows | macOS | Ubuntu 24.04 |
| --- | --- | --- | --- |
| Fenster | `obit-gui.exe`, nur x64 | `Obituary.app`, universell für Intel und Apple Silicon | ein `.deb`-Paket, nur amd64 |
| Symbol | Infobereich | Menüleiste | Symbol im Infobereich; braucht einen Desktop mit StatusNotifierItem |
| Symbol bei der Anmeldung | nach `obit init` standardmäßig an | Schalter in den Settings | Schalter in den Settings |
| Prüfung auf Abgelaufenes | residenter Prozess des Symbols | LaunchAgent | systemd |
| Desktop-Benachrichtigungen | ja | ja | ja |
| Kontextmenü | im Explorer | Finder: Schnellaktionen | Nautilus: Skripte · Dolphin |
| Datums-Tags | ja | Finder | Dolphin |
| Papierkorbmodus | ja | nein, nur Friedhof | ja |
| Installiert mit | dem Installationsskript, Scoop, winget oder dem Archiv | dem DMG | dem `.deb` |
| Erster Start | ein von Hand heruntergeladenes Archiv kann eine SmartScreen-Warnung auslösen | braucht eine manuelle Freigabe, siehe [unten](#erste-offnung-einer-unsignierten-app) | öffnet sich direkt |
| Voraussetzungen | die WebView2-Runtime | 1.6.0 braucht macOS 26 (behoben in 1.6.1) | GTK 3 und WebKitGTK 4.1, von apt installiert |

Windows nutzt eine Aufgabe bei der Anmeldung, macOS einen Benutzer-LaunchAgent und Linux einen systemd-Benutzerdienst in der grafischen Sitzung. Nach einem Absturz erfolgt der Neustart nach 60 Sekunden; beim dritten Absturz innerhalb von 15 Minuten enden die Versuche.

Bei aktiver Desktop-Überwachung verarbeitet der Tray-Prozess fällige Einträge sofort; ein unabhängiger Auftrag prüft stündlich nach. Ohne Tray-Überwachung für dieses Datenverzeichnis läuft der Auftrag alle fünf Minuten.

Unter Windows werden nur alte Aufgaben migriert, die der für diese Installation erzeugten Definition entsprechen. Das Original wird für eine Rückkehr zum alten Stand gesichert. Geänderte oder deaktivierte Aufgaben bleiben erhalten.

## Kleines Fenster und Menüsymbol unter macOS

Lade das universelle DMG (`obituary_<Version>_darwin_universal.dmg`) und `checksums.txt` von der Release-Seite herunter, prüfe den SHA-256, öffne das DMG, ziehe Obituary.app in „Programme“, wirf das DMG aus und öffne die App aus „Programme“. Ziehe sie zuerst heraus, weil der Autostart-Eintrag bei der Anmeldung festhält, wo die App liegt, und eine direkt aus dem DMG gestartete App nach dem Auswerfen dort nicht mehr ist.

Die App enthält das Fenster, das Menüsymbol und die dazugehörigen Programme `obit` und `obitd`. Schaltest du in den Settings des Fensters „Start the tray at sign-in“ ein, wird der eigene Benutzer-LaunchAgent von Obituary installiert, damit das Menüsymbol bei der Anmeldung erscheint; schaltest du es aus, wird nur dieser Eintrag entfernt.

Mit `obit daemon enable` installierst du den unabhängigen Prüfauftrag. Er läuft auch bei geschlossenem Fenster oder beendeter Menüleisten-App weiter.

Wenn du `Obituary.app` entfernst, bleiben deine Daten standardmäßig unter `~/Library/Application Support/obituary/`: das Register in `store.json`, die Einstellungen in `config.toml` und die auf diesem Volume begrabenen Dateien in `graveyard/`.

### Erste Öffnung einer unsignierten App

Das Release hat keine Developer-ID-Signatur und ist nicht notarisiert, daher hält macOS es beim ersten Mal an. Stelle sicher, dass der Download von einer vertrauenswürdigen Release-Seite stammt, prüfe den SHA-256 und öffne die App dann einmal ganz normal. Sagt das System, es könne den Entwickler nicht überprüfen, öffne Systemeinstellungen, dann Datenschutz & Sicherheit, suche unten auf der Seite Obituary, wähle „Trotzdem öffnen“, bestätige noch einmal und öffne die App erneut. Schalte Gatekeeper dafür nicht global aus und entferne das Quarantäne-Attribut nicht rekursiv. Meldet das System, die App sei beschädigt oder enthalte Schadsoftware, oder verbietet eine Unternehmensrichtlinie das Öffnen, höre hier auf.

### 1.6.0 braucht macOS 26

Das macOS-Fenster in 1.6.0 braucht macOS 26. Unter macOS 15 und früher meldet das System, die App sei mit dieser Version nicht kompatibel, und sie öffnet sich nicht. In 1.6.1 ist das behoben.

## Ubuntu 24.04 (amd64)

Lade das Desktop-Paket `obituary_<Version>_linux_amd64.deb` und `checksums.txt` von der Release-Seite herunter. Führe im Download-Verzeichnis die folgenden Befehle aus und ersetze `VERSION` durch die heruntergeladene Version; das Paket wird nur installiert, wenn die Prüfung gelingt:

```bash
file=obituary_VERSION_linux_amd64.deb
grep -F "  $file" checksums.txt | sha256sum --check --strict - && sudo apt install "./$file"
```

Öffne danach Obituary aus der Anwendungsliste. Das Fenster nutzt GTK 3 und WebKitGTK 4.1, und apt installiert beides mit dem Paket. Die Programme liegen in `/usr/libexec/obituary/`, das nicht in deinem PATH steht; um `obit` im Terminal zu nutzen, installiere die Befehlszeile gesondert mit dem Installationsskript.

Das Symbol im Infobereich braucht einen Desktop, der StatusNotifierItem bereitstellt, und GNOME tut das standardmäßig nicht. Dort öffnest du dasselbe Fenster aus der Anwendungsliste, ohne Erweiterung, aber es gibt kein Symbol. Damit das Symbol bei deiner nächsten Anmeldung erscheint, schalte in den Settings „Start the tray at sign-in“ ein; schaltest du es aus, wird der Autostart-Eintrag gelöscht, den Obituary geschrieben hat. Zum Deinstallieren siehe [Deinstallation](./uninstall).

## Das Symbol im Infobereich

`obit init` richtet ein Symbol im Infobereich ein, das bei der Anmeldung erscheint. Sein Menü bietet:

- die zuletzt begrabenen Einträge; klicke einen an, um ihn wiederzubeleben;
- Einträge, die innerhalb der nächsten 24 Stunden ablaufen;
- die automatische Bereinigung anhalten oder fortsetzen;
- „Open Obituary“ zum Öffnen des kleinen Fensters; fehlt `obit-gui.exe` im Verzeichnis von `obitd.exe`, erklärt ein Hinweis das Problem;
- Beenden (die geplante Aufgabe läuft weiter).

## Das kleine Fenster

`obit-gui.exe` hat fünf Seiten: Expiring, Graveyard, For you, Settings und Logs. Du öffnest das Fenster über „Obituary“ im Startmenü, die Desktop-Verknüpfung oder „Open Obituary“ im Tray-Menü.

- Die obere Leiste der Startseite zeigt „Auto registration: off“ oder „on“. Ein Klick öffnet die Einstellungen. Oben auf For you und unter Settings kannst du zwischen aus (Standard: nur du und der Agent registrieren), abgestuft, vollautomatisch und immer nachfragen wählen; siehe [Was registriert wird](./auto-register). Weiter unten stehen unter „Handled automatically“ die ohne Rückfrage registrierten Einträge. Je Eintrag kannst du die Registrierung mit einem Klick rückgängig machen (die Frist endet, der Eintrag bleibt an Ort und Stelle) oder die Lebensdauer verlängern.
- Fällige Einträge, die nicht begraben werden konnten, etwa weil ein Programm die Datei offen hält, erscheinen unter „Burials that failed“ auf For you. Dort stehen Grund und nächster Versuch. Die Wartezeit zwischen den Versuchen wächst. Nach wiederholten Fehlern wartet Obituary auf dich: „Retry“ versucht es sofort erneut, „Stop tracking“ lässt den Eintrag an seinem Platz und beendet die Überwachung.
- Läuft Docker nicht, ist „Approve“ bei Containervorschlägen nicht verfügbar. Die Karte nennt den Grund. Sobald Docker läuft, kannst du den Vorschlag genehmigen.
- Wenn du einer `.obitrules` vertraust, gilt das nur für die im Fenster angezeigte Version.
- Fenster, Infobereich und Kontextmenü verwenden `[ui] language` aus `config.toml` (`auto`, `zh-CN` oder `en`; `auto` übernimmt die Windows-Sprache). Du kannst die Sprache auch in den Einstellungen ändern.
- Das kleine Fenster gibt es nur für x64. Es benötigt die WebView2-Laufzeit, die zu Windows 11 gehört und auf den meisten Windows-10-Rechnern vorhanden ist.

### Updates

Unter Settings → About and updates kannst du eine neuere stabile Version installieren. Obituary schließt Fenster und laufendes Tray, aktualisiert über den ursprünglichen Installationsweg (Scoop, WinGet oder Skript) und versucht anschließend, Tray und Fenster wieder zu öffnen. Fehler und Protokollpfade stehen auf derselben Seite.

Eine selbst entpackte Kopie wird ebenfalls an Ort und Stelle aktualisiert; ihre Schaltfläche heißt „Download and restart to update“. Obituary lädt die Version von GitHub herunter (vom Spiegelserver in Festlandchina, wenn GitHub nicht innerhalb von zehn Sekunden antwortet), prüft die SHA-256-Prüfsumme und schließt erst dann das Fenster und ersetzt die eigenen Programme von Obituary in diesem Ordner (`obit`, `obitd` und `obit-gui`). Alle anderen Dateien im Ordner bleiben unverändert, auch PATH und das Startmenü werden nicht angefasst. Die bisherigen Programme bleiben einen Tag in einem versteckten Ordner `.obituary-update-…` am selben Ort und kommen dann auf den Friedhof, wo sie noch wiederbelebt werden können; schlägt ein Schritt fehl, auch wenn ein neues `obit` nicht startet, wird die vorherige Version zurückgelegt. Voraussetzung sind eine stabile Version (kein Entwicklungs-Build), alle Programme der Version als normale Dateien im Ordner und ein Ordner, in den der aktuelle Benutzer ohne Administratorrechte schreiben kann. Trifft etwas davon nicht zu, nennt die Seite den Grund, und du kannst den Ersatz von der Release-Seite herunterladen. Im Terminal erledigt `obit update --apply` dasselbe. „What changed“ zeigt die Hinweise in der Sprache des Fensters, auf Englisch, wenn es keine Übersetzung gibt.

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

Erscheinen nach einem Upgrade alle Explorer-Menüeinträge in einer anderen unterstützten Sprache, funktioniert das Menü weiterhin. `obit doctor` meldet dies als Hinweis (JSON: `installation: label-language-mismatch`). `obit init --context-menu --yes` stellt die Beschriftung auf die aktuelle Oberflächensprache um.

Im Finder stehen die Aktionen unter Schnellaktionen, in Nautilus unter Skripte → Obituary. Dolphin kann in statischen Dienstmenüs eine freie Fläche nicht von einem einzelnen ausgewählten Ordner unterscheiden; in beiden Fällen fehlen die Obituary-Aktionen. In Dolphin ist das Untermenü für eine Datei oder eine Mehrfachauswahl verfügbar, und Ordnerrichtlinien erfordern mindestens zwei ausgewählte Ordner. Für einen einzelnen ausgewählten Ordner die Befehlszeile oder Nautilus verwenden. Bestehende Menüs mit `obit init --context-menu --yes` aktualisieren.

## Symbole und Menüs

`obit init` richtet die folgenden Elemente standardmäßig ein. Unter Settings → Icons and menus lassen sie sich einzeln schalten:

| Einstellung | Wirkung |
|---|---|
| Tray-Symbol | Zeigt Obituary im Infobereich. Die Bereinigung läuft auch bei ausgeblendetem Symbol. |
| Taskleistensymbol | Bestimmt, ob das geöffnete Fenster einen Platz in der Taskleiste belegt. |
| Startmenü | „Obituary“ im Startmenü. |
| Desktop-Verknüpfung | "Obituary" auf dem Desktop |
| Kontextmenü | Das Menü „Add to Obituary“ im Explorer. |

Willst du sie schon bei der Installation nicht haben, gib `obit init --no-context-menu` an, um das Kontextmenü wegzulassen, oder `--no-shortcuts`, um die Verknüpfungen im Startmenü und auf dem Desktop wegzulassen. Dieselben Optionen gelten beim Deinstallieren: `obit init --uninstall --no-shortcuts` lässt die vorhandenen Verknüpfungen stehen und entfernt den Rest wie gewohnt, und `--no-context-menu` verhält sich beim Kontextmenü genauso.

Wie der Explorer die Begräbnisdaten anzeigt, steht unter [Ordnerrichtlinien](./folder-policies#dates-in-explorer).
