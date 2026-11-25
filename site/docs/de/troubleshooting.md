# Fehlerbehebung

Wenn etwas nicht stimmt, beginne mit `obit doctor`. Der Befehl gibt zuerst eine kurze Zusammenfassung aus: Die erste Zeile sagt, ob alles in Ordnung ist oder wie viele Probleme gefunden wurden, darunter steht für jeden Bereich (Installation, Hintergrundbetrieb, Agentenanbindung, Kontextmenü, Daten, Hooks usw.) eine Zeile mit `PASS`, `WARN` oder `FAIL`. Unter einem Bereich, der Aufmerksamkeit braucht, stehen der Grund und ein Befehl zur Behebung, den du direkt kopieren kannst. Bei jedem Aufruf wird außerdem der vollständige Bericht als eigenständige HTML-Datei im Ordner `reports` des Datenverzeichnisses gespeichert (er bleibt auf deinem Rechner und wird nach 1 Tag begraben); die letzten Zeilen nennen den Ort. `obit doctor --open` öffnet ihn im Browser, `obit doctor --verbose` gibt die vollständige Textfassung im Terminal aus: eine Zeile pro Prüfung, jeweils mit `PASS`, `INFO`, `WARN` oder `FAIL` am Anfang, gefolgt vom Grund und davon, was zu tun ist, dazu die Fähigkeiten jedes Agenten. Sobald es ein `FAIL` gibt, endet der Befehl mit einem Exit-Code ungleich null. Mit `--json` gibst du das Ergebnis weiter (das Format ist unverändert). Die weiter unten auf dieser Seite zitierten Zeilen wie `path` oder `obitd` sind die einzelnen Prüfungen, die `--verbose` und der Bericht zeigen.

Jeder Abschnitt unten folgt derselben Reihenfolge: was du siehst, warum es so ist, was du tun kannst und woran du erkennst, dass es behoben ist.

Erscheinen nach einem Upgrade alle Explorer-Menüeinträge in einer anderen unterstützten Sprache, funktioniert das Menü weiterhin. `obit doctor` meldet dies als Hinweis (JSON: `installation: label-language-mismatch`). `obit init --context-menu --yes` stellt die Beschriftung auf die aktuelle Oberflächensprache um.

## Die Shell findet `obit` nicht

Das Terminal meldet `command not found`, oder PowerShell erkennt `obit` nicht. Das Programm liegt auf der Platte, aber sein Ordner steht nicht in deinem PATH. Das Windows-Installationsskript trägt `%LOCALAPPDATA%\Programs\obituary` in deinen Benutzer-PATH ein, aber ein bereits geöffnetes Terminal sieht den neuen PATH nicht. `install.sh` legt die Programme in `~/.local/bin` ab und ändert den PATH nie; auf manchen Systemen steht dieser Ordner ohnehin nicht im PATH. Das Ubuntu-`.deb` legt sie in `/usr/libexec/obituary/` ab, das ebenfalls nicht im PATH steht.

Unter Windows schließt du das Terminal und öffnest ein neues. Unter macOS und Linux trägst du `~/.local/bin` in die Konfiguration deiner Shell ein: bei zsh schreibst du `export PATH="$HOME/.local/bin:$PATH"` in `~/.zshrc` und öffnest ein neues Terminal. Hast du nur das `.deb` installiert, nimm den vollen Pfad `/usr/libexec/obituary/obit`, oder installiere die Befehlszeile mit dem Installationsskript gesondert.

Behoben ist es, wenn `obit --version` eine Version ausgibt und die Zeile `path` in `obit doctor` nicht mehr `WARN` ist.

## `obit doctor` findet `obitd` nicht

Eine Zeile `FAIL obitd` in `obit doctor` bedeutet, dass das Hintergrundprogramm `obitd` nicht neben `obit` liegt. Obituary sucht es im selben Ordner, daher reicht es nicht, nur `obit` zu kopieren. Lege `obit` und `obitd` zusammen ab, oder führe die Installation für die Art, wie du installiert hast, erneut aus. Zeigt `obit doctor` in dieser Zeile `PASS`, ist es behoben.

## Eine heruntergeladene Datei besteht die Prüfsumme nicht

Nutzt du das Installationsskript, macht es diese Prüfung selbst und bricht ab, ohne etwas zu installieren, wenn der Hash nicht stimmt. Von Hand musst du nur prüfen, wenn du ein Archiv, ein DMG oder ein `.deb` selbst heruntergeladen hast. Eine Abweichung bedeutet meist, dass der Download unvollständig ist, die Datei unterwegs verändert wurde oder `checksums.txt` aus einem anderen Release stammt, denn die Hashes gehören zu je einem Release.

Lade das Paket und `checksums.txt` noch einmal von derselben Release-Seite herunter (aus Festlandchina vom Mirror) und prüfe erneut; passt es dann immer noch nicht, führe es nicht aus. Ersetze `VERSION` und den Dateinamen durch die deiner Datei. Unter Linux:

```bash
file=obituary_VERSION_linux_amd64.deb
grep -F "  $file" checksums.txt | sha256sum --check --strict -
```

Unter macOS ersetzt du in der letzten Zeile `sha256sum --check --strict -` durch `shasum -a 256 -c -`. In Windows PowerShell:

```powershell
$file = "obituary_VERSION_windows_amd64.zip"
(Get-FileHash $file -Algorithm SHA256).Hash -eq ((Select-String -Path checksums.txt -SimpleMatch $file).Line -split '\s+')[0]
```

Stimmen die Hashes überein, geben Linux und macOS den Dateinamen gefolgt von `OK` aus, und PowerShell gibt `True` aus.

## Windows meldet „Der Computer wurde durch Windows geschützt“

Windows SmartScreen kann ein Archiv stoppen, das du im Browser von Hand heruntergeladen hast, weil die Programme von Obituary noch nicht code-signiert sind. Mit Scoop, winget oder dem Installationsskript passiert das selten. Prüfe zuerst wie oben den SHA-256; stimmt er, wähle „Weitere Informationen“ und dann „Trotzdem ausführen“. Kannst du ihn nicht prüfen, oder meldet Windows Schadsoftware statt nur eines unbekannten Herausgebers, führe die Datei nicht aus.

## Das Windows-Fenster öffnet sich nicht

Ein Klick auf Obituary im Startmenü oder im Infobereich bewirkt nichts, oder es heißt, eine Laufzeitkomponente fehle. Das Fenster wird von der Microsoft-Edge-WebView2-Runtime gezeichnet. Windows 11 bringt sie mit und die meisten Windows-10-Rechner haben sie schon, aber wo sie fehlt, kann das Fenster nicht starten. Lade die WebView2-Runtime (die Evergreen-Version) von Microsoft herunter, installiere sie und klicke noch einmal auf Obituary; öffnet sich das Fenster, ist es behoben.

Das Fenster gibt es bisher nur für x64. Das ARM64-Archiv für Windows enthält kein `obit-gui.exe`, dort nutzt du also die Befehlszeile. Zeigt „Open Obituary“ im Menü des Infobereichs-Symbols eine Benachrichtigung, dass das Fensterprogramm nicht gefunden wird, liegt `obit-gui.exe` nicht neben `obitd.exe`, und genau so ist das ARM64-Archiv aufgebaut. Unter x64 legst du die beiden in denselben Ordner.

## macOS: Die App meldet, sie sei mit dieser Version nicht kompatibel

Das macOS-Fenster in 1.6.0 (`Obituary.app` im DMG) braucht macOS 26. Unter macOS 15 und früher erscheint beim Öffnen die Meldung, die App sei mit dieser macOS-Version nicht kompatibel, und sie startet nicht. Das ist ein Verpackungsfehler in 1.6.0, behoben in 1.6.1. Bis dahin braucht das Fenster macOS 26. Das Problem betrifft nur das Fenster im DMG. Mit 1.6.1 oder neuer öffnet sich die App, und daran erkennst du es.

## macOS: „Entwickler kann nicht überprüft werden“

Die App im DMG hat keine Developer-ID-Signatur und ist nicht notarisiert, deshalb hält Gatekeeper sie beim ersten Mal an. Öffne sie nach der Prüfung des SHA-256 einmal ganz normal. Sagt macOS, der Entwickler könne nicht überprüft werden, gehe zu Systemeinstellungen, dann Datenschutz & Sicherheit, suche unten auf der Seite Obituary, wähle „Trotzdem öffnen“ und bestätige noch einmal. Die Schritte stehen unter [Erste Öffnung einer unsignierten App](./desktop#erste-offnung-einer-unsignierten-app). Schalte Gatekeeper dafür nicht global aus. Meldet macOS, die App sei beschädigt oder enthalte Schadsoftware, ist das etwas anderes, und du solltest sie nicht zulassen.

## Das Symbol startet nicht bei der Anmeldung (macOS, Ubuntu)

Nach der Anmeldung ist kein Obituary-Symbol in der Menüleiste oder im Infobereich. Der Start bei der Anmeldung beruht auf einem Autostart-Eintrag: unter macOS dem Benutzer-LaunchAgent `site.woodfish.obituary.tray.supervised`, unter Ubuntu `obituary-tray.service`. Ist der Schalter in den Settings des Fensters aus, wurde die App verschoben oder wurde der Schalter eingeschaltet, während die App direkt aus dem DMG lief, ist der im Eintrag gespeicherte Pfad veraltet, und bei der Anmeldung wird nichts gefunden.

Öffne das Fenster und schalte in den Settings „Start the tray at sign-in“ aus und wieder ein, damit Obituary den Eintrag neu schreibt. Unter macOS sollte `launchctl list | grep obituary` nun `site.woodfish.obituary.tray.supervised` zeigen; unter Ubuntu sollte `systemctl --user status obituary-tray.service` den Dienststatus anzeigen.

## Ubuntu: Das Fenster öffnet sich nicht, oder es gibt kein Symbol im Infobereich

Ein Klick auf Obituary in der Anwendungsliste bewirkt nichts, was meist heißt, dass GTK 3 oder WebKitGTK 4.1 fehlt. `apt install ./….deb` installiert sie mit; sie fehlen nur, wenn sie später entfernt wurden oder die Installation mittendrin abgebrochen ist. Unter Ubuntu 24.04 heißt das GTK-3-Paket `libgtk-3-0t64`; der Name ohne `t64`, `libgtk-3-0`, existiert in diesem Release nicht, ein aus einer älteren Anleitung kopierter Befehl schlägt also fehl:

```bash
sudo apt update
sudo apt install libgtk-3-0t64 libwebkit2gtk-4.1-0 xdg-utils
```

Öffne Obituary erneut aus der Anwendungsliste; erscheint das Fenster, ist es behoben. Öffnet sich das Fenster, aber es gibt kein Symbol im Infobereich, liegt das daran, dass das Symbol einen Desktop braucht, der einen StatusNotifierItem-Host bereitstellt, und GNOME tut das standardmäßig nicht. Dasselbe Fenster kannst du ohne Erweiterung aus der Anwendungsliste öffnen; für das Symbol installierst du in GNOME eine Erweiterung, die es unterstützt, etwa AppIndicator.

## Abgelaufenes wird nicht begraben

`obit daemon status` zeigt den Zustand des Auftrags. Mit `obit reap` lassen sich fällige Einträge sofort verarbeiten.

Bei aktiver Desktop-Überwachung verarbeitet der Tray-Prozess fällige Einträge sofort; ein unabhängiger Auftrag prüft stündlich nach. Ohne Tray-Überwachung für dieses Datenverzeichnis läuft der Auftrag alle fünf Minuten.

`obit doctor` nennt den zuständigen Dienst, die letzten Fehler und den Grund für das letzte Beenden. Es unterscheidet zwischen Wartezeit bis zum Neustart, Pause, absichtlichem Stopp und fehlender oder ausgefallener Überwachung.

Führe `obit daemon enable` aus, um die Aufgabe zu installieren (unter Windows wird dabei auch das Symbol im Infobereich für die Anmeldung eingerichtet), und `obit sweep`, wenn du die Bereinigung sofort willst. Unter Linux ohne systemd-Benutzerdienst, etwa in manchen Containern und minimalen Systemen, kann `obit daemon enable` sie nicht installieren; nimm stattdessen cron und führe alle 5 Minuten `obitd --once --home <Datenverzeichnis> --config-dir <Konfigurationsverzeichnis>` aus, beide Verzeichnisse nennt dir `obit config`. Sagt die Zeile `runner` von `obit daemon status` nicht mehr `off`, ist es behoben; unter Linux siehst du den Timer auch mit `systemctl --user list-timers | grep obituary`.

## Etwas ist abgelaufen, wurde aber nicht begraben

Der Eintrag steht auf der Seite „For you“ des Fensters unter „Burials that failed“, mit einem Grund daneben. Oft ist eine Datei in einem Editor, einem Synchronisierungsclient oder einem anderen Programm geöffnet und liegt noch da, wo sie war. Begraben heißt, die Datei zu verschieben, und das scheitert, wenn sie in Benutzung ist oder dir die Rechte fehlen. Obituary erzwingt nichts: Das Original bleibt unberührt, es versucht es nach einer Weile erneut und wartet jedes Mal länger, und nach mehreren Fehlschlägen hält es an und wartet auf dich.

Schließe, was die Datei festhält, und klicke auf „Retry“, oder führe `obit sweep` aus. Soll sich nichts mehr darum kümmern, klicke auf „Stop tracking“: Die Datei bleibt, wo sie ist, und der Countdown endet. Verschwindet der Eintrag aus „Burials that failed“ und taucht in `obit graveyard` auf, wurde er begraben.

## Eine wiederbelebte Datei hat einen anderen Namen

`obit resurrect` hat keinen Fehler gemeldet, aber die Datei kam als `report (resurrected).txt` zurück, weil am ursprünglichen Ort schon etwas mit diesem Namen lag. Das Wiederbeleben überschreibt nie; ist der Name vergeben, legt es die Datei im selben Ordner unter einem anderen Namen ab, und beide Kopien bleiben unversehrt. Vergleiche sie, behalte die gewünschte und benenne sie um.

## Die Konfigurationsdatei enthält einen Fehler

`obit config` meldet `config file has errors`, gefolgt von einem Dateipfad, einer Zeilennummer und einem Schlüsselnamen. In `config.toml` steht ein Schlüssel, den Obituary nicht kennt, oder ein unzulässiger Wert. Obituary ignoriert das nicht stillschweigend, denn sonst bliebe bei `retention = 30` die Aufbewahrung von 7 Tagen in Kraft, während du glaubst, sie habe sich geändert. Korrigiere in der genannten Zeile Schreibweise, Anführungszeichen oder Dauer und führe `obit config` erneut aus; beanstandet es nichts mehr und ist die Zeile `config` in `obit doctor` `PASS`, ist es behoben. Wie jede Einstellung geschrieben wird, steht in der [Konfigurationsreferenz](./config).

## Ein Hook hat keine Wirkung

Nach `obit init` wurde ein Agent nicht am Löschen einer Datei gehindert, oder es scheint nichts zu passieren. Es gibt drei häufige Ursachen: Der Agent wurde nicht neu gestartet und arbeitet noch mit der alten Konfiguration; die Anbindung dieses Agenten wurde in einem anderen Geltungsbereich installiert (global oder Projekt); oder in der Umgebung des Agenten ist `OBIT_DISABLED=1` (schaltet alle Hooks ab) oder `OBIT_GUARD_RM=0` (schaltet den Löschschutz ab) gesetzt.

Sieh zuerst nach, was installiert ist:

```bash
obit init -g --show        # was global installiert ist
obit init --show           # was im aktuellen Projekt installiert ist
obit doctor --verbose      # wie jeder Agent angebunden ist
```

Ist es nicht installiert, installiere es mit `obit init -g --agent <Name>`, und mit `--dry-run` siehst du vorher den Plan. Starte danach den Agenten neu; in Codex führst du außerdem `/hooks` aus und vertraust den Hooks von Obituary. Um zu sehen, was der Hook tatsächlich empfängt, setze `OBIT_HOOK_AUDIT=1` in dem Terminal, aus dem du den Agenten startest, nutze den Agenten eine Weile und führe dann `obit hook audit` aus. Abgelehnte Löschbefehle brauchen diesen Schalter nicht, sie werden immer protokolliert. Die Fehler des Hooks selbst landen in `hook.log` im Datenverzeichnis, und die Zeile `hook-log` von `obit doctor` zählt die Fehler der letzten 24 Stunden.

Ist die Zeile des Agenten in `obit doctor` (zum Beispiel `claude`) `PASS` und lautet `integration is current`, ist er installiert; nutzt du den Löschschutz, sollte eine Zeile wie `claude-guard-rm` `registered: yes` zeigen. Führe `obit doctor` in demselben Terminal aus, aus dem du den Agenten startest: Hat eine Umgebungsvariable die Hooks abgeschaltet, wird diese Zeile zu `WARN` und nennt die Variable.

## Der Vorschlag für einen Docker-Container lässt sich nicht genehmigen

„Approve“ bei einem Container-Vorschlag ist im Fenster ausgegraut, und die Karte sagt „Docker is not running: start Docker to decide on this proposal.“ Einen Container zu registrieren heißt, Docker nach seiner vollständigen ID und seiner Erstellungszeit zu fragen, und das geht nicht, solange Docker aus ist. Starte Docker Desktop oder die Docker Engine und genehmige erneut; lässt sich „Approve“ anklicken und zeigt `obit docker list` den Container, hat es geklappt.

Läuft ein Container ab, hält Obituary ihn nur an und behält seine beschreibbare Schicht, und beim Wiederbeleben wird er neu gestartet; ein angehaltener Container wird erst nach der Aufbewahrungsfrist gelöscht.

## Die Bestattung eines Docker-Volumes ist unbestätigt

Docker-Ereignisse werden vor und nach dem Löschen geprüft. Lässt sich die Löschung nicht bestätigen, bleibt das geprüfte Archiv erhalten. Prüfe `obit doctor`, statt von einer erfolgreichen Bestattung auszugehen. Jede Warnung unter `docker-volume-recovery` nennt einen zu prüfenden Eintrag. Prüfe das Volume wie angegeben auf dem ursprünglichen Docker-Daemon. Nicht verbuchte Archive lassen sich weder mit `obit resurrect` noch mit `obit purge` bearbeiten. [Docker-Container und Volumes](./docker)

## Ein Update schlägt fehl, oder Downloads aus Festlandchina sind langsam

`obit update --check` meldet einen Fehler, oder das Installationsskript ist langsam oder scheitert; am wahrscheinlichsten erreicht dieser Rechner GitHub nicht. Antwortet GitHub nur zögerlich, wechseln `obit update --check` und das Installationsskript von selbst zum Mirror in Festlandchina. Willst du den Mirror von Anfang an nutzen, führe unter macOS und Linux `bash install.sh --mirror cn` aus, unter Windows `.\install.ps1 -Mirror cn`. Lädst du von Hand herunter, nimm Archiv und `checksums.txt` aus demselben Release und von derselben Quelle. Gibt `obit update --check` eine Zeile `Latest:` aus, ist es behoben.

## Eine abgelaufene Datei bleibt liegen

Schließen Sie Programme, die die Datei geöffnet haben. Fehlen nur Prozessinformationen, können Sie die Datei im Modus `propose` einzeln bestätigen. Im automatischen Modus müssen alle Prozesse des Dateieigentümers prüfbar sein. Unlesbare Prozesse anderer Benutzer einschließlich root werden nur bei eigenen Dateien ohne Schreibrecht für Gruppe oder andere ignoriert. Mit `propose` prüfen Sie jede Datei einzeln. Die Freigabe akzeptiert Lücken in der Prozessliste. Eine nachweislich geöffnete Datei bleibt mit Begründung liegen. `obit doctor --verbose` · `obit watch --list`
