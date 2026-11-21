# Konfigurationsreferenz

Obituary liest `config.toml` aus seinem Konfigurationsverzeichnis. Alle Schlüssel sind optional; fehlende Schlüssel verwenden die folgenden Standardwerte. `obit config` zeigt Pfad und wirksame Werte.

## Schlüssel

| Schlüssel | Standardwert | Werte oder Format | Wirkung |
| --- | --- | --- | --- |
| `codex_windows_shell` | nicht gesetzt | `bash`, `powershell` | Legt die von Codex unter Windows verwendete Shell fest; unklare Löschbefehle werden ohne diese Angabe durchgelassen. |
| `agent_shells.<agent>` | nicht gesetzt | `bash`, `powershell` | Legt die Windows-Shell für einen Hook-Agenten fest. |
| `graveyard.mode` | `graveyard` | `graveyard`, `recycle` | Verschiebt abgelaufene Einträge in den Friedhof oder Papierkorb. |
| `graveyard.retention_days` | `7` | Ganzzahl von `1` bis `3650` | Tage, während der ein Eintrag wiederhergestellt werden kann. |
| `watch.stable_window` | `10m` | Go-Dauer wie `30s`, `10m`, `1h` | Wartet vor der Bewegung durch eine Ordnerregel auf Stabilität. Muss positiv sein. |
| `explorer.enabled` | `true` | `true`, `false` | Schreibt Datumsmarkierungen im Explorer. |
| `explorer.everywhere` | `false` | `true`, `false` | Zeigt Datumsmarkierungen in allen unterstützten Ordnern. |
| `explorer.in_git` | `false` | `true`, `false` | Zeigt Datumsmarkierungen in Git-Worktrees. |
| `ui.language` | `auto` | `auto`, `en`, `zh-CN` | Sprache von CLI, Fenster und Tray. |
| `ui.tray` | `true` | `true`, `false` | Aktiviert den Tray- oder Menüleisten-Eintrag. |
| `ui.taskbar` | `true` | `true`, `false` | Gibt dem Windows-Fenster eine eigene Taskleisten-Schaltfläche. |
| `hooks.session_summary` | `false` | `true`, `false` | Zeigt beim Start einer Sitzung eine Zusammenfassung. |
| `hooks.session_cleanup` | `true` | `true`, `false` | Räumt in dieser Sitzung erstellte, noch nicht kategorisierte Dateien auf. |
| `proposals.mode` | `off` | `off`, `tiered`, `auto`, `ask` | Legt fest, welche automatischen Registrierungen auf Genehmigung warten. |
| `update.check` | `true` | `true`, `false` | Prüft automatisch auf stabile Versionen. |
| `telemetry.enabled` | `true` | `true`, `false` | Erlaubt Nutzungsstatistiken; unabhängig von der Update-Prüfung. |

`agent_shells` ist eine nach Agentennamen geordnete Tabelle, etwa `[agent_shells] codex = "powershell"`. `codex_windows_shell` ist die ältere Codex-Schreibweise; widersprüchliche Werte werden abgelehnt.

## Formate und Fehler

Dauern verwenden die Go-Syntax und können `ns`, `us`, `ms`, `s`, `m` und `h` kombinieren, etwa `1h30m`. Die Aufbewahrung muss zwischen `1` und `3650` Tagen liegen. `graveyard.mode`, `ui.language` und `proposals.mode` akzeptieren nur die angegebenen Werte.

Unbekannte Schlüssel werden nicht stillschweigend ignoriert. Der Fehler nennt Pfad, Zeile und vollständigen Schlüssel und schlägt bei ähnlicher Schreibweise eine Alternative vor. Ungültiges TOML lässt `obit config` ebenfalls fehlschlagen.

## Häufige Befehle

```powershell
obit config
obit config --create
obit config --language en
obit config telemetry off
obit config telemetry on
```

`--create` schreibt die kommentierte Standardvorlage nur bei fehlender Datei. `--language` akzeptiert `auto`, `en` und `zh-CN`. `telemetry on|off` ändert nur `telemetry.enabled` und nicht `[update] check`.
