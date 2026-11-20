# Häufige Fragen

## Warum nicht einfach löschen?

Löschst du eine Datei endgültig und brauchst sie doch noch, kannst du sie nicht zurückholen. Obituary begräbt sie stattdessen auf dem Friedhof, standardmäßig für sieben Tage. Bis dahin kannst du sie mit `obit resurrect` jederzeit zurückholen. Erst nach Ablauf der Frist wird sie dauerhaft gelöscht. Greift ein Agent innerhalb der sieben Tage auf sie zu, erfährt er sofort die Grabnummer und den Befehl zum Wiederbeleben (siehe [Hinweise auf begrabene Dateien](./grave-notices)).

## Läuft Obituary unter macOS und Linux?

Ja. Beide Systeme haben Kommandozeilen- und Hintergrundprogramme für x64 und ARM64, mit Friedhof, Wiederbelebung und automatischem Begraben. Fällige Einträge prüft unter macOS launchd, unter Linux ein systemd-Benutzertimer. Auch die Agenten-Hooks stoßen bei Bedarf eine Prüfung an. Siehe [Schnellstart](./index). Papierkorbmodus, kleines Fenster, Tray und Kontextmenüs gibt es nur unter Windows.

## Windows meldet, der Computer sei geschützt worden. Was tun?

Obituary ist noch nicht codesigniert. Erscheint die Meldung nach dem Download des Archivs im Browser, wähle „Weitere Informationen → Trotzdem ausführen“. Bei Scoop, winget und dem Installationsskript erscheint sie normalerweise nicht. Alle drei prüfen vorher die SHA-256-Prüfsumme des Archivs.

## Gibt es ein kleines Fenster auf ARM64?

Nein. `obit-gui.exe` gibt es nur für x64. Der ARM64-Build enthält die Kommandozeile und das Tray.

## Wird Code registriert, den ich noch nicht committet habe?

Nein. Die automatische Registrierung ist standardmäßig aus. Registriert wird nur, was der Agent mit `obit mark` oder du über das Kontextmenü hinzufügst. Ist sie eingeschaltet, erfasst sie nur unversionierte Einträge im Git-Arbeitsbaum. Neue Dateien, auf die keine Regel passt (etwa noch nicht committeter Code), werden nicht aufgelistet. Abhängigkeiten, Build-Ausgaben und Caches bleiben unberührt. Was über eine Regel oder einen Vorschlag registriert wurde, wird nicht begraben, wenn es bei Ablauf bereits committet ist. Siehe [Was registriert wird](./auto-register).

## Was passiert, wenn Git ausfällt oder meine `.gitignore` sehr viele Dateien ausschließt?

Obituary fragt vor der Registrierung bei Git nach, ob es das Objekt bereits verfolgt. Kann Git nicht antworten, etwa weil es fehlt, das Zeitlimit abläuft oder das Repository beschädigt ist, wird nichts registriert. Ordner außerhalb eines Git-Repositorys werden nie automatisch registriert. Nur du oder der Agent können ihnen mit `obit mark` eine Lebensdauer geben. Eine weit gefasste `.gitignore` lässt lediglich mehr Dateien unversioniert; automatisch registriert wird weiterhin nur nach den eingebauten Regeln und einer von dir als vertrauenswürdig bestätigten `.obitrules`.

## Kann ein Agent meine Dateien als seine eigenen ausgeben und so den Vorschlag umgehen?

Nein. Standardmäßig ist die automatische Registrierung aus und nichts wird ohne deine Zustimmung automatisch eingetragen. Im abgestuften Modus werden nur Dateien direkt registriert, die der Agent nachweislich in seiner Sitzung erstellt hat: Der Hook hat den Schreibvorgang erfasst, im Anfangssnapshot fehlen sie und ihr Erstellungszeitpunkt liegt nach dem Sitzungsbeginn. Schon vorhandene oder von anderen Programmen erzeugte Dateien werden lediglich vorgeschlagen. Selbst wenn ein Agent dazu gebracht wird, einen alten Ordner passend zu einer Regel umzubenennen, bleibt dessen Erstellungszeit gleich; auch er wird nur vorgeschlagen. Von Git verfolgte Dateien werden in keinem Modus automatisch registriert. Jede direkte Registrierung wird nachträglich gemeldet und lässt sich mit `obit spare` rückgängig machen. Begrabene Einträge lassen sich sieben Tage lang wiederbeleben.

## Kann ein Agent Vorschläge für mich genehmigen?

Nein. Es gibt keine MCP-Werkzeuge, um Vorschläge anzunehmen oder abzulehnen, `.obitrules` zu vertrauen, Friedhofseinträge dauerhaft zu löschen oder die Konfiguration einschließlich des Vorschlagsmodus zu ändern. Der Agent kann dir den Vorschlag nur weitergeben.

## Funktioniert `obit resurrect` im Papierkorbmodus?

Nein. Bei `mode = "recycle"` in `config.toml` landen begrabene Einträge im Papierkorb des Systems. Stelle sie von dort wieder her.

## Wie schalte ich eine Funktion vorübergehend ab?

- Hooks: `OBIT_DISABLED=1`
- Löschschutz: `OBIT_GUARD_RM=0`
- Automatische Bereinigung: "Pause" im Tray-Menü

## Wo sind die Daten?

`%LOCALAPPDATA%\obituary` enthält das Register, das Auditprotokoll, das Daemonprotokoll und den Friedhof dieses Laufwerks. Die Konfiguration steht in `%APPDATA%\obituary\config.toml`. Siehe [Funktionsweise](./how-it-works#where-the-data-lives).

## Löscht die Deinstallation den Friedhof?

Nein. `obit uninstall` behält Friedhof und Register standardmäßig bei. Gib `--purge-data` nur an, wenn du beides löschen möchtest. Siehe [Deinstallation](./uninstall).
