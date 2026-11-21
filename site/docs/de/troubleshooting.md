# Fehlerbehebung

Beginnen Sie mit `obit doctor`. Der Befehl prüft Installation, Integrationen, Hintergrunddienste und Daten, ohne Dateien zu ändern. Für mehr Details verwenden Sie `--json` oder `--verbose`.

## PATH und Prüfsummen

**Symptom:** `obit` wird nicht gefunden oder die Prüfsumme des Archivs stimmt nicht.

**Lösung:** Laden Sie `checksums.txt` aus derselben Version und führen Sie `sha256sum --check --strict checksums.txt` aus. In PowerShell vergleichen Sie `Get-FileHash .\obit-*.zip -Algorithm SHA256`. Führen Sie abweichende Archive nicht aus und öffnen Sie nach einer PATH-Änderung ein neues Terminal.

## Desktop

Bei Windows SmartScreen prüfen Sie zuerst SHA-256 und wählen dann „Weitere Informationen → Trotzdem ausführen“. Fehlt WebView2, installieren Sie Microsoft Edge WebView2 Runtime. Das Windows-Fenster unterstützt derzeit nur x64.

Die macOS-App ist nicht Developer-ID-signiert und nicht notarisiert. Prüfen Sie die Prüfsumme und erlauben Sie den ersten Start unter „Systemeinstellungen → Datenschutz & Sicherheit → Trotzdem öffnen“. Deaktivieren Sie Gatekeeper nicht und erlauben Sie keine App mit Schadsoftware- oder Beschädigungswarnung. Bei LaunchAgent helfen `obit daemon status` und `launchctl list | grep obituary`.

Für das Fenster unter Ubuntu 24.04 werden GTK 3 und WebKitGTK 4.1 benötigt:

```bash
sudo apt update
sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0
obit doctor
```

Prüfen Sie die Timer mit `systemctl --user status obituary.timer`, `systemctl --user list-timers | grep obituary` und `crontab -l`.

## Hintergrund, Updates und Dateien

`obit daemon status` zeigt den Zustand, `obit sweep` startet einen sofortigen Durchlauf und `obit daemon enable` aktiviert die Planung. Für den chinesischen Spiegel gilt `bash install.sh --mirror cn`. Mischen Sie keine Archive und Prüfsummen verschiedener Versionen.

Nach einer Umbenennung eines WSL-Laufwerks prüfen Sie den echten Windows-Pfad. Gesperrte Dateien bleiben erhalten und werden erneut versucht. TOML-Fehler nennt `obit config` mit Datei und Zeile.

## Integrationen und Container

Bei einem inaktiven Hook führen Sie `obit init --show`, `obit init --dry-run` und `obit doctor --verbose` aus und prüfen, dass `OBIT_DISABLED=1` nicht gesetzt ist. Der Hook-Audit nutzt `OBIT_HOOK_AUDIT=1 obit hook audit`. Starten Sie Docker vor der Containerfreigabe. Eine Wiederherstellung überschreibt kein vorhandenes Ziel, sondern verwendet einen konfliktfreien Namen.
