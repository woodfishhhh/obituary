# Deinstallation

Die Deinstallation hat zwei Schritte: Zuerst entfernt Obituary, was es eingerichtet hat, dann löschst du die Programme selbst. Friedhof und Register bleiben standardmäßig erhalten, sodass Dateien, die du begraben hast, auch danach noch wiederbelebt werden können.

```powershell
obit uninstall --dry-run      # listet auf, was entfernt würde; ändert nichts
obit uninstall                # zeigt den Plan und wendet ihn nach deiner Bestätigung an
```

Hast du unter Windows mit dem Skript installiert, genügt ein Befehl:

```powershell
.\install.ps1 -Uninstall      # führt obit uninstall --yes für dich aus und entfernt dann die Programmdateien, den PATH-Eintrag, den es angelegt hat, und seine Startmenü-Verknüpfung
```

## Was entfernt wird

`obit uninstall` arbeitet auf Benutzerebene und am aktuellen Git-Projekt: die Hooks, MCP und Skill jedes Agenten, die geplante Aufgabe (die Aufgabenplanung unter Windows, ein launchd-Benutzeragent unter macOS, ein systemd-Benutzer-Timer unter Linux), der residente Prozess und das Symbol im Infobereich sowie die Ordnerrichtlinien (die Dateien unter einer Richtlinie bleiben, wo sie sind). Unter Windows entfernt es außerdem den Autostart-Eintrag bei der Anmeldung, das Explorer-Kontextmenü und die Datumsmarkierungen im Explorer.

Es fasst nur an, was Obituary selbst geschrieben hat. Ein Eintrag, den du bearbeitet hast, oder einer, den eine andere Installation hinterlassen hat, bleibt unverändert, und der Plan nennt, welcher und warum. Wo es kein Terminal gibt (ein Skript, die Befehlszeile eines Agenten), füge `--yes` hinzu; ohne diese Option bricht der Befehl ab und sagt dir, dass er ein Terminal braucht.

Drei Dinge nimmt er dir nicht ab. Die Programmdateien löschst du so, wie du installiert hast (siehe unten). Hast du projektbezogene Hooks in anderen Projekten installiert, führe in jedem dieser Projekte `obit uninstall` aus, bevor du die Programme entfernst, sonst schlagen die Hooks dort fehl, weil `obit` fehlt. Und unter macOS und Linux steht der Autostart-Eintrag des Infobereichs nicht auf der Liste: Schalte vor dem Deinstallieren in den Settings des Fensters „Start the tray at sign-in“ aus, wodurch der Eintrag entfernt wird, den Obituary geschrieben hat. Unter Windows übernimmt `obit uninstall` den Autostart des Symbols selbst, aber die Verknüpfungen im Startmenü und auf dem Desktop, die `obit init` angelegt hat, stehen nicht auf seiner Liste; `obit init --uninstall` entfernt sie, außer du gibst `--no-shortcuts` an.

## Friedhof, Register und deine Konfiguration

Drei Dinge bleiben standardmäßig erhalten: die Dateien auf dem Friedhof, das Register samt Audit-Protokoll und `config.toml`. `obit uninstall` sagt dir, wie viel noch auf dem Friedhof liegt und wo, und eine spätere Neuinstallation nimmt alles wieder auf. Nutzt du den Papierkorbmodus, liegt Abgelaufenes bereits im Papierkorb des Systems; die Deinstallation rührt ihn nicht an, und du stellst es aus dem Papierkorb wieder her, weil `obit resurrect` es nicht erreicht.

Willst du wirklich alles loswerden, füge `--purge-data` hinzu:

```powershell
obit uninstall --purge-data
```

Zuerst werden alle Einträge des Friedhofs aufgelistet, und nach der Bestätigung des Deinstallationsplans wird ein zweites Mal gefragt. Danach, noch bevor etwas entfernt wird, prüft Obituary alles, was es anfassen will: ob das Datenverzeichnis ihm selbst gehört, ob es jede darin liegende Datei kennt und ob sich der Friedhofsordner und jedes zu bereinigende Grab verifizieren lassen. Besteht eine Prüfung nicht, hält es dort an und sagt, welche; Friedhof, Register und Dienste bleiben, wie sie waren, sodass eine Bereinigung nie halb erledigt ist. Erst wenn alles besteht, löscht es diese Dateien endgültig, danach das Register, das Audit-Protokoll und die übrigen Dateien, die Obituary führt. In einem Skript überspringen `--yes --purge-data` zusammen beide Rückfragen.

Gelöscht werden nur Dateien, die es erkennt. Andere Dateien, Links oder ein Unterordner, den Obituary nicht kennt, lassen die Prüfung anhalten, und es nennt, was es gefunden hat. `config.toml` gehört nie zur Bereinigung, und du musst sie nicht verschieben, auch dann nicht, wenn sie im selben Ordner wie das Register liegt (der Standard unter macOS): Nach der Bereinigung enthält dieser Ordner nur noch `config.toml`. Liegt die Konfiguration an einem anderen Ort, wird das ganze Datenverzeichnis gelöscht.

## Die Programme entfernen

Wie du die Programme entfernst, hängt davon ab, wie du sie installiert hast.

| Installiert mit | Entfernen mit |
| --- | --- |
| dem Windows-Installationsskript | `.\install.ps1 -Uninstall` |
| Scoop | `obit uninstall`, dann `scoop uninstall obituary` |
| winget | `obit uninstall`, dann `winget uninstall woodfishhhh.Obituary` |
| dem macOS-Fenster (DMG) | `obit uninstall` und „Start the tray at sign-in“ ausschalten, dann Obituary.app aus „Programme“ in den Papierkorb ziehen |
| Homebrew | `obit uninstall`, dann `brew uninstall --cask obituary` |
| `install.sh` unter macOS oder Linux | `obit uninstall`, dann `obit` und `obitd` aus `~/.local/bin` löschen |
| dem Ubuntu-Desktop-Paket (`.deb`) | „Start the tray at sign-in“ ausschalten und das Symbol im Infobereich beenden, dann `sudo apt remove obituary-desktop` |

Die macOS-App und das Ubuntu-`.deb` bringen jeweils ein eigenes `obit` mit, legen es aber beide nicht in deinen PATH. Hast du die Befehlszeile nicht separat installiert, starte es vor dem Deinstallieren mit dem vollen Pfad: `/Applications/Obituary.app/Contents/MacOS/obit` in der App, `/usr/libexec/obituary/obit` beim `.deb`. `apt remove` löscht die Programme in diesem Ordner und lässt deinen Friedhof, deine Einstellungen und jede anders installierte Befehlszeile unangetastet.

## Wo die Daten liegen

| | Daten (Register, Friedhof, Protokolle) | Konfiguration (`config.toml`) |
| --- | --- | --- |
| Windows | `%LOCALAPPDATA%\obituary` | `%APPDATA%\obituary` |
| macOS | `~/Library/Application Support/obituary` | derselbe Ordner |
| Linux | `~/.local/share/obituary` | `~/.config/obituary` |

Sind unter Linux `XDG_DATA_HOME` oder `XDG_CONFIG_HOME` gesetzt, ist es jeweils der Ordner `obituary` darunter. Die Umgebungsvariablen `OBIT_HOME` und `OBIT_CONFIG_DIR` verlegen Daten und Konfiguration an einen anderen Ort, und `obit config` zeigt, welche gerade verwendet werden. Auf dem Laufwerk, das das Datenverzeichnis enthält, ist der Friedhof dessen Ordner `graveyard`; auf anderen Laufwerken ist es ein versteckter Ordner `.obituary` im Stammverzeichnis des Laufwerks, damit das Begraben immer eine Umbenennung auf einem einzigen Laufwerk ist und nie ein Kopieren.
