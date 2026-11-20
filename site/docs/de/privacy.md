# Datenschutz

Das Register, der Friedhof, die Protokolle und die Einstellungen von Obituary bleiben auf deinem Computer. Obituary lädt keine Absturzberichte, Dateien, Pfade oder Dateiinhalte hoch.

## Nutzungsstatistik

Die Statistik ist standardmäßig aktiviert. Sie dient ausschließlich dazu, aktive Installationen zu zählen und die Verteilung der Versionen zu ermitteln. Bei tatsächlicher Nutzung kann Obituary höchstens einmal in 24 Stunden eine zufällige Installations-ID, die Version, das Betriebssystem, die Architektur, die Oberflächensprache und den Installationskanal senden. Es sendet keine Dateien, Pfade oder Befehle und speichert weder IP-Adressen noch User-Agent-Header. Mit `obit config telemetry off` oder `OBIT_TELEMETRY=0` schaltest du die Meldungen ab. In CI-Umgebungen und bei `DO_NOT_TRACK=1` werden keine Meldungen gesendet.

Der Server speichert weder IP-Adressen noch User-Agents noch ursprüngliche Installations-IDs. Ein mit einem separaten Schlüssel berechneter Hashwert ordnet Meldungen derselben Installation zu, nicht einer Person. Die täglichen Aktivitätsdaten verfallen nach 400 Tagen. Zur Vermeidung von Doppelzählungen bleiben langfristig nur der Hashwert und das Datum der ersten und letzten Beobachtung erhalten. Erfolgreiche Meldungen werden pro Kanal nur als Summe gezählt. Nur die Verantwortlichen für die Wartung können das zugangsgeschützte Dashboard einsehen.

Beim Abschalten werden die lokale Installations-ID und ausstehende Meldungen gelöscht. Die Suche nach Updates hat einen eigenen Schalter.

## Suche nach einer neuen Version

- Höchstens einmal am Tag liest Obituary öffentliche Informationen zur neuesten Veröffentlichung des Projekts auf `https://api.github.com`. Antwortet GitHub nicht, liest es `https://cn.obituary.woodfish.site/download/latest.json` vom Spiegelserver in Festlandchina.
- Die Anfrage enthält nur die üblichen Angaben: deine IP-Adresse und einen User-Agent mit der Version von Obituary, etwa `Obituary/1.0.0`.
- Du kannst die automatische Suche jederzeit in den Einstellungen des Fensters mit „Check for updates automatically“ abschalten oder Folgendes in `%APPDATA%\obituary\config.toml` eintragen:

  ```toml
  [update]
  check = false
  ```

  Danach prüft Obituary nur noch auf Updates, wenn du selbst `obit update --check` ausführst.

## Installation und Downloads

- Scoop, winget und das Installationsskript laden die Archive von [GitHub Releases](https://github.com/woodfishhhh/obituary/releases). Es gilt die [Datenschutzerklärung von GitHub](https://docs.github.com/site-policy/privacy-policies/github-general-privacy-statement).
- Mit `-Mirror cn` kommen die Dateien von `cn.obituary.woodfish.site`. Zur Fehlersuche führt dieser Server gewöhnliche Zugriffsprotokolle mit Uhrzeit, IP-Adresse und angeforderter URL.
- GitHub Actions überträgt die Dateien bei der Veröffentlichung auf den Spiegelserver. Dieser prüft sie und stellt sie lokal bereit, ohne deine Downloadanfrage an GitHub weiterzuleiten.

## Agenten anbinden

`obit init` ändert nur die Konfigurationsdateien der Agenten auf deinem Computer und zeigt dir vorher den Plan. Es sendet nichts an die Anbieter der Agenten oder an andere Stellen.

## Website

- <https://obituary.woodfish.site/> verwendet keine Analyse- oder Tracking-Skripte. In Festlandchina liefert unser Server die Website aus, andernorts GitHub Pages. Beide führen gewöhnliche Zugriffsprotokolle.
- Eine auf der Website hinterlegte E-Mail-Adresse dient ausschließlich zur Ankündigung neuer Plattformen und Versionen. Sie liegt auf unserem eigenen Server und wird nicht an Dritte weitergegeben. Jede E-Mail enthält einen Abmeldelink.

## Kontakt

Fragen zum Datenschutz kannst du in einem [GitHub-Issue](https://github.com/woodfishhhh/obituary/issues) stellen. Sicherheitsprobleme meldest du bitte vertraulich, wie in [SECURITY.md](https://github.com/woodfishhhh/obituary/blob/main/SECURITY.md) beschrieben.
