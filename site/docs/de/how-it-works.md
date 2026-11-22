# So funktioniert Obituary

## Das Leben einer Datei

1. **Registrieren**: Gib der Datei oder dem Ordner eine Lebensdauer. Der Agent verwendet `obit add debug.log 2h`, du wählst „Add to Obituary“ im Explorer-Kontextmenü. Siehe [Was registriert wird](./auto-register).
2. **Ablaufen**: `obit list` zeigt die verbleibende Zeit. Vor dem Ablauf kannst du mit `obit extend` verlängern oder mit `obit spare` die Registrierung aufheben und die Datei behalten.
3. **Begraben**: Bei Ablauf wird der Eintrag auf den Friedhof verschoben (`obit graveyard`) und dort standardmäßig sieben Tage aufbewahrt.
4. **Wiederbeleben oder dauerhaft löschen**: Innerhalb der Frist bringt `obit resurrect <id>` den Eintrag an seinen ursprünglichen Ort zurück, ohne etwas zu überschreiben. Erst nach der Frist wird er dauerhaft gelöscht (`obit purge`).

Weißt du noch nicht, ob du eine Datei brauchst? Mit `obit bury` kannst du sie sofort begraben und sieben Tage lang unverändert wiederbeleben.

## Wo liegt der Friedhof?

Der Friedhof liegt immer auf demselben Laufwerk wie die Datei: auf dem Laufwerk des Datenverzeichnisses in dessen Friedhof, auf anderen Laufwerken im versteckten Verzeichnis `.obituary` an der Laufwerkswurzel. Das Begraben ist daher nur eine Umbenennung, geht schnell und kann nicht auf halbem Weg stehen bleiben.

Die Aufbewahrungsfrist lässt sich konfigurieren. Du kannst Einträge stattdessen direkt in den Papierkorb des Systems verschieben lassen. In diesem Modus stellst du sie aus dem Papierkorb wieder her, nicht mit `obit resurrect`:

```toml
[graveyard]
mode = "graveyard"      # or "recycle": straight to the system Recycle Bin
retention_days = 7      # days the graveyard keeps things
```

Die Konfiguration steht in `%APPDATA%\obituary\config.toml`. `obit config` zeigt sie an; `obit config --create` schreibt eine kommentierte Datei mit den Standardwerten.

## Warum keine anderen Dateien gelöscht werden

- **Dauerhaft gelöscht wird nur beim Leeren eines Grabs.** Jedes Begräbnis erhält einen eigenen Grabordner; dessen Datei-ID wird gespeichert. Vor dem Löschen oder Wiederbeleben prüft Obituary, ob der Ordner noch derselbe ist, genau einen Eintrag enthält und der Eintrag tatsächlich das begrabene Objekt ist. Selbst bei einem beschädigten Register oder einem durch einen Link ersetzten Friedhof wird nichts außerhalb des Friedhofs gelöscht.
- **Wiederbelebt wird nur am ursprünglichen Ort.** Ist der Name belegt, wird nichts überschrieben. Der Eintrag wird daneben unter einem Namen wie `name (resurrected).ext` wiederhergestellt. Beide bleiben erhalten, die Ausgabe nennt den tatsächlichen Pfad. Ein gelöschter übergeordneter Ordner wird neu angelegt. Ist ein übergeordneter Ordner inzwischen eine Junction oder ein symbolischer Link, zeigt eine Junction auf ein anderes Ziel oder verweist der Laufwerksbuchstabe auf ein anderes Volume, wird die Wiederbelebung abgelehnt. Das Grab bleibt unverändert.
- **Entscheidend ist die Datei, nicht ihr Name.** Jeder Eintrag enthält die Seriennummer des Volumes und die 128-Bit-Datei-ID. Umbenannte oder verschobene Dateien werden wiedergefunden; eine neue Datei am alten Pfad bleibt unangetastet. Direkt vor dem Löschen wird die Identität nochmals geprüft. Dateien mit mehr als einem Hardlink werden abgelehnt.
- **Geschützte Pfade.** Laufwerkswurzeln, `C:\Windows`, Program Files, ProgramData, das Benutzerprofil, Desktop, Dokumente, Downloads und ähnliche Ordner sowie deren übergeordnete Verzeichnisse sind geschützt. Das gilt auch bei Umleitungen. Nur ihren Inhalten kannst du eine Lebensdauer geben.
- **Git-Repositorys.** Enthält ein Ordner in beliebiger Tiefe `.git`, benötigen `obit add` und `obit bury` die Option `--force`. Bei Ablauf wird dies erneut geprüft. Kann die Prüfung nicht abschließen, wird der Ordner nicht begraben.
- **Im Zweifel wird nicht gelöscht.** Ein Eintrag wird nur aufgegeben, wenn die Datei nachweislich nicht mehr existiert. Bei verweigertem Zugriff, einem entfernten Volume und ähnlichen Fehlern versucht es Obituary später erneut. Der Abstand beginnt bei einer Minute und steigt bis auf einen Tag. Nach 12 aufeinanderfolgenden Fehlern wartet Obituary auf dich. Den Grund findest du in `obit list` und auf For you unter „Burials that failed“. `obit extend <id> 1m` („Retry“) versucht es erneut, `obit spare <id>` („Stop tracking“) belässt den Eintrag am bisherigen Ort. Vor dem Begraben eines großen Ordners sucht Obituary bis zu 30 Sekunden lang nach Git-Repositorys. Nur eine abgeschlossene Suche ohne Treffer erlaubt das Begraben. Andernfalls wird die Suche später fortgesetzt; ein Ordner mit Repository bleibt erhalten.
- **Alle Vorgänge werden protokolliert.** Jede Registrierung, Verlängerung, Aufhebung und Löschung steht im Auditprotokoll (`obit log`).

## Hintergrundbetrieb

Bei aktiver Desktop-Überwachung verarbeitet der Tray-Prozess fällige Einträge sofort; ein unabhängiger Auftrag prüft stündlich nach. Ausgaben ohne Tray prüfen weiterhin alle fünf Minuten.

Windows nutzt eine Aufgabe bei der Anmeldung, macOS einen Benutzer-LaunchAgent und Linux einen systemd-Benutzerdienst in der grafischen Sitzung. Nach einem Absturz erfolgt der Neustart nach 60 Sekunden; beim dritten Absturz innerhalb von 15 Minuten enden die Versuche.

Agenten-Hooks führen zu Sitzungsbeginn und -ende ebenfalls einen Durchlauf aus; `obit reap` startet sofort einen.

```powershell
obit daemon enable              # tray at login plus the fallback scheduled task (--dry-run lists what would be installed)
obit daemon status              # current mode and the next expiry (--json)
obit daemon disable             # remove the scheduled task and login start, and stop the running daemon (--dry-run previews)
```

Abfragen wie `list`, `graveyard`, `stats` und `log` lesen nur. Sie begraben nichts.

## Speicherort der Daten {#where-the-data-lives}

In `%LOCALAPPDATA%\obituary` liegen `store.json` (Register), `audit.jsonl` (Auditprotokoll), `obitd.log` (Daemonprotokoll) und `graveyard\` (Friedhof dieses Laufwerks).

Mit `OBIT_HOME` (Daten), `OBIT_CONFIG_DIR` (Konfiguration) und `OBIT_USER_HOME` (Home-Verzeichnis) kannst du die jeweiligen Speicherorte ändern.
