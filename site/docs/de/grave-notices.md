# Hinweise auf begrabene Dateien

Wer Dateien zuerst begräbt, muss sie später wiederfinden können. Greift ein Agent mit einem Werkzeug auf einen begrabenen, noch wiederherstellbaren Pfad oder auf einen Eintrag in einem begrabenen Ordner zu, nennt Obituary ihm:

- die Grabnummer;
- den Zeitpunkt des Begrabens;
- die verbleibende Zeit für die Wiederherstellung;
- `obit resurrect <id>`.

## Nur ein Hinweis

- Der Werkzeugaufruf wird nie blockiert oder geändert.
- Jedes Grab wird einmal pro Sitzung erwähnt.
- Claude Code und Codex erhalten den Hinweis vor der Ausführung des Werkzeugs. Für andere Agenten zeigt `obit init --show` den Zeitpunkt an.
- Ansonsten stört Obituary nicht: Der Agent muss beim Start nur wissen, dass Obituary verfügbar ist, und keine Liste lesen.

Bei einer bestehenden Installation genügt es, `obit init` erneut auszuführen, um die Hinweise zu aktivieren.

## Sitzungszusammenfassung (optional)

Um Tokens zu sparen, erhält der Agent zu Sitzungsbeginn nur Vorschläge, die deine Entscheidung erfordern, und die gerade automatisch registrierten Einträge. Für eine Zusammenfassung aktiviere „Session summary“ in den Einstellungen des kleinen Fensters oder setze:

```toml
[hooks]
session_summary = true
```

Der Agent sieht dann auch, welche Einträge des Projekts in den nächsten 24 Stunden ablaufen und welche kürzlich begrabenen Einträge sich noch wiederbeleben lassen. Temporäre Verzeichnisse aus `obit mktemp` sind ausgenommen.

## Den Friedhof selbst abfragen

```powershell
obit graveyard           # what is in the graveyard and when it will be purged
obit graveyard --here    # this folder only
obit resurrect <id>      # put it back; if the name is taken, it comes back renamed, never overwriting
obit resurrect --last    # undo the last deletion: everything one obit bury / obit rm buried comes back
```
