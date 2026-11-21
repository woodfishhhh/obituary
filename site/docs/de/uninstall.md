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
