# Deinstallation

```powershell
obit uninstall --dry-run      # see what would be removed
obit uninstall                # remove everything Obituary set up, after you confirm
.\install.ps1 -Uninstall      # if you used the install script: also remove the program files, PATH entry and Start menu shortcut
```

`obit uninstall` entfernt nur, was Obituary selbst eingerichtet hat: Agenten-Hooks, MCP und Skill für das Benutzerkonto und das aktuelle Git-Projekt sowie den Autostart bei der Anmeldung, die geplante Aufgabe, den laufenden Daemon, das Kontextmenü, die Datumsanzeige im Explorer und die Ordnerrichtlinien.

## Friedhof und Register bleiben erhalten

Standardmäßig bleiben der Friedhof und das Register erhalten. Der Befehl zeigt an, wo der Friedhof liegt und wie viel er enthält. Nur wenn du beides löschen möchtest, gib `--purge-data` an. Dann wird zuerst der Inhalt des Friedhofs aufgelistet und eine weitere Bestätigung verlangt.

## Hooks in anderen Projekten

Hast du in anderen Projekten projektspezifische Hooks installiert, führe dort jeweils `obit uninstall` aus, bevor du das Programm entfernst. Andernfalls funktionieren diese Hooks nicht mehr, sobald `obit` fehlt.

## Scoop und winget

Führe zuerst `obit uninstall` aus und anschließend den Deinstallationsbefehl des verwendeten Paketmanagers.
## Deinstallationsumfang und Datenorte

`obit uninstall --dry-run` zeigt entfernbarere Elemente für den aktuellen Benutzer und das aktuelle Git-Projekt. Der Befehl entfernt Hooks, MCP, Skills, Tray, Autostart und Zeitpläne, ändert aber keine anderen Projekte; dort muss er separat ausgeführt werden.

| Element | Windows | macOS | Linux |
| --- | --- | --- | --- |
| Programm | `obit`, `obitd`, optional `obit-gui` | `/Applications/Obituary.app` oder CLI-Verzeichnis | CLI-Verzeichnis, Desktop als `.deb` |
| Hook, MCP, Skill | Agent-Konfiguration | Agent-Konfiguration | Agent-Konfiguration |
| Planung | Aufgabe, residenter Prozess und Tray | LaunchAgent und Menüleiste | systemd-Timer und XDG-Autostart |
| Projekt | Obituary-Teile in `.claude/` und `.codex/` | Projektkonfiguration | Projektkonfiguration |

Standarddaten liegen unter Windows in `%LOCALAPPDATA%\obituary` und `%APPDATA%\obituary`, unter macOS in `~/Library/Application Support/obituary` und unter Linux in `$XDG_DATA_HOME/obituary` bzw. `~/.local/share/obituary`. `OBIT_HOME`, `OBIT_CONFIG_DIR` und `OBIT_USER_HOME` können Daten, Konfiguration und Benutzerverzeichnis umleiten; auf demselben Volume kann `.obituary` liegen.

Friedhof, Ledger, Audit-Protokoll und Konfiguration bleiben standardmäßig. `--purge-data` löscht nur verifizierte eigene Friedhofsdaten und behält unbekannte Dateien. Im recycle-Modus wird aus dem Papierkorb wiederhergestellt, nicht mit `resurrect`.
