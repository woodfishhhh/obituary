# Schnellstart

Obituary gibt Dateien und Ordnern eine Lebensdauer. Wenn sie abläuft, werden sie auf einem Friedhof begraben; während der Aufbewahrungszeit (standardmäßig 7 Tage) können Sie sie jederzeit wiederbeleben, und erst danach werden sie wirklich gelöscht. Es wurde für KI-Agenten entwickelt: Debugskripte, Protokolle, Screenshots und Testausgaben eines Agenten haben ab dem Erstellen eine Lebensdauer. Niemand muss sich an das Aufräumen erinnern, und das `rm -rf` eines Agenten zerstört nichts mehr dauerhaft.

## 1. Installieren

Windows 10 / 11, macOS und Linux (x64 und ARM64):

| | Windows | macOS | Linux |
|---|---|---|---|
| Installieren mit | Scoop, dem Skript `install.ps1` (winget wird noch geprüft) | Homebrew, dem Skript `install.sh` | dem Skript `install.sh` |
| Ablauf alle 5 Minuten prüfen | residenter Prozess in der Taskleiste, Aufgabenplanung als Ersatz | launchd-Agent des Benutzers | `systemd --user`-Timer; Benutzer-Crontab ohne systemd |
| CLI, Friedhof und Wiederbelebung, automatische Registrierung (optional), Löschschutz, Agent-Integration (Hooks, Skill, MCP), Docker-Container, Ordnerrichtlinien | ja | ja | ja |
| Taskleiste, kleines Fenster, Startmenü und Desktopverknüpfungen | ja | stattdessen die CLI: `obit list`, `obit graveyard`, `obit resurrect` | wie bei macOS |
| Kontextmenü und Begräbnisdaten im Dateimanager | Explorer | Finder: Schnellaktionen, Datums-Tags | Nautilus: Skripte · Dolphin: Menü, Datums-Tags |
| Papierkorbmodus | ja | nur Friedhof | nur Friedhof |

**Am einfachsten: Lassen Sie Ihren Agenten die Arbeit erledigen.** Senden Sie diesen Satz an den Agenten, den Sie verwenden (Claude Code, Codex, Cursor usw.). Er installiert Obituary, verbindet es mit sich selbst und hält an, wenn Sie entscheiden müssen.

```text
Read https://obituary.woodfish.site/agent-install.md and follow it to install Obituary and connect it to yourself.
```

**Manuell:** Wählen Sie unten Ihr Betriebssystem aus und führen Sie `obit doctor` aus, um die Installation zu prüfen.

### macOS / Linux

Unter macOS lässt sich Obituary mit Homebrew in einer Zeile installieren:

```bash
brew install --cask woodfishhhh/tap/obituary
```

Oder laden Sie das Installationsskript herunter, lesen Sie es und führen Sie es aus (macOS und Linux):

```bash
curl -fsSL https://raw.githubusercontent.com/woodfishhhh/obituary/main/install.sh -o install.sh
bash install.sh
# Custom directory: --install-dir "$HOME/bin"; preview only: --dry-run
```

Das Skript installiert standardmäßig die neueste Version (`--version <version>` wählt eine andere). Das Archiv wird gleichzeitig in vier Teilen heruntergeladen (einige Routen begrenzen die Geschwindigkeit je Verbindung) und fällt bei Problemen auf eine einzelne Verbindung zurück. Es erkennt Betriebssystem und Architektur, prüft den SHA-256-Wert aus derselben Downloadquelle und installiert `obit` und `obitd` in `~/.local/bin`. Befolgen Sie die PATH-Hinweise und führen Sie anschließend `obit --version` aus. Für den Spiegel in Festlandchina verwenden Sie `--mirror cn`; das Skript steht unter `https://cn.obituary.woodfish.site/install.sh`.

Führen Sie Obituary bei der ersten Verwendung unter macOS mit `obit --version` und `obitd --help` aus. Wenn Gatekeeper eine Genehmigung verlangt, öffnen Sie Systemeinstellungen → Datenschutz & Sicherheit, wählen Sie für jede ausführbare Datei mit geprüfter Herkunft „Trotzdem öffnen“ und versuchen Sie den Befehl erneut.

Unter WSL gehören Projekte und Agent-Konfiguration in das native Linux-Dateisystem, etwa nach `~/projects`. DrvFS-Laufwerke wie `/mnt/c` unterstützen möglicherweise nicht das sichere Umbenennen bei der Installation. Verwenden Sie ausschließlich die Windows-Version, um Dateien auf diesen Laufwerken eine Lebensdauer zu geben, sie zu begraben oder wiederherzustellen.

Liefert das overlayfs eines Docker-Containers keine dauerhaften Dateikennungen, lehnt `obit mark` die Registrierung ab und lässt die Datei an ihrem Platz. Für Lebensdauer und Wiederherstellung legen Sie die Dateien und das Obituary-Datenverzeichnis auf einem Volume oder Bind-Mount mit dauerhaften Kennungen ab, etwa auf ext4.

### Windows

::: code-group

```powershell [Scoop]
scoop bucket add obituary https://github.com/woodfishhhh/scoop-bucket
scoop install obituary
```

```powershell [winget]
# In review, available once approved
winget install woodfishhhh.Obituary
```

```powershell [Install script]
# Install script: download it, read it, then run it
# (installs to %LOCALAPPDATA%\Programs\obituary, adds it to your user PATH, creates a Start menu shortcut)
irm https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1 -OutFile install.ps1
.\install.ps1          # installs the latest release; -WhatIf previews, -NoPath leaves PATH alone
```

:::

- Alle drei Methoden laden dasselbe Archiv aus dem GitHub-Release herunter und prüfen vor dem Entpacken den SHA-256-Wert; bei einer Abweichung wird die Installation beendet.
- Codesignatur: [SignPath.io](https://about.signpath.io/) stellt die kostenlose Codesignatur bereit, das Zertifikat kommt von der [SignPath Foundation](https://signpath.org/). Siehe die [Richtlinie zur Codesignatur](code-signing) und die [Datenschutzerklärung](privacy).
- Obituary ist noch nicht codesigniert. Wenn Sie das Archiv im Browser herunterladen, zeigt Windows möglicherweise „Windows hat Ihren PC geschützt“ an; wählen Sie „Weitere Informationen → Trotzdem ausführen“. Scoop, winget und das Installationsskript lösen diese Meldung normalerweise nicht aus.
- Das kleine Fenster `obit-gui.exe` benötigt die WebView2-Laufzeitumgebung (in Windows 11 enthalten und auf den meisten Windows-10-Rechnern vorhanden) und ist nur für x64 verfügbar; der ARM64-Build enthält die Befehlszeile und die Taskleiste.
Prüfen Sie anschließend die Installation:

```powershell
obit doctor              # health check: short summary, full HTML report (--open, --verbose, --json)
```

## 2. Agenten verbinden

```powershell
obit init -g                 # Claude Code, all projects; also the Skill (and desktop integration on Windows)
obit init -g --agent codex   # Codex
```

Starten Sie den Agenten danach neu. Weitere Agenten, den MCP-Server und den Agent Skill finden Sie unter [Agenten verbinden](./agents).

Unter macOS / Linux verwendet Claude Code `~/.claude`, Codex `~/.codex` und gemeinsame Skills `~/.agents/skills`. Überschreibungen durch `CLAUDE_CONFIG_DIR` und `CODEX_HOME` werden ebenfalls berücksichtigt.

Bei aktiver Desktop-Überwachung verarbeitet der Tray-Prozess fällige Einträge sofort; ein unabhängiger Auftrag prüft stündlich nach. Ohne Tray-Überwachung für dieses Datenverzeichnis läuft der Auftrag alle fünf Minuten.

```bash
obit daemon enable --dry-run
obit daemon enable
obit doctor
```

macOS verwendet den launchd des angemeldeten Benutzers, Linux `systemd --user`. Unter Linux ohne systemd planen Sie `obitd --once` alle 5 Minuten in der Benutzer-Crontab ein und geben die absoluten Pfade zu `obitd`, zum Datenverzeichnis `--home` und zum Konfigurationsverzeichnis `--config-dir` an. Agent-Hooks starten außerdem eine Hintergrundprüfung, sobald eine Aufgabe fällig ist.

## 3. Häufige Befehle

Ersetzen Sie `<id>` durch die Nummer aus einem Beleg von `obit add` oder `obit bury`, oder durch die Nummer aus `obit list` / `obit graveyard`.

```powershell
obit add debug.log 2h        # bury it in 2 hours (alias: mark)
obit list                    # what is left and for how long
obit extend <id> 1d          # one more day
obit bury old-build          # bury right away
obit graveyard               # what is in the graveyard and when it will be purged
obit resurrect <id>          # put it back, never overwriting
obit resurrect --last        # undo the last deletion
```

Alles Weitere finden Sie unter [Befehle](./commands).

## Danach

| Das möchten Sie wissen | Lesen |
|---|---|
| Wie Registrierung, Begräbnis, Wiederbelebung und dauerhaftes Löschen funktionieren und wo sich der Friedhof befindet | [Funktionsweise](./how-it-works) |
| Claude Code, Codex und andere Agenten verbinden | [Agenten verbinden](./agents) |
| Was aufgenommen wird: Agent-Registrierungen, Kontextmenü, optionale automatische Registrierung | [Was aufgenommen wird](./auto-register) |
| Das `rm -rf` eines Agenten in ein rückgängig machbares Begräbnis verwandeln | [Löschschutz](./delete-guard) |
| Was geschieht, wenn ein Agent auf eine begrabene Datei zugreift | [Meldungen aus dem Friedhof](./grave-notices) |
| Lebensdauern für Ordner wie Downloads und Desktop | [Ordnerrichtlinien](./folder-policies) |
| Obituary ohne Befehlszeile verwenden | [Fenster, Taskleiste und Kontextmenü](./desktop) |
| Docker-Container, die in einer Sitzung gestartet wurden | [Docker-Container](./docker) |
| Jeder Befehl | [Befehle](./commands) |
| Obituary entfernen | [Deinstallieren](./uninstall) |
| Alles Weitere | [FAQ](./faq) |
