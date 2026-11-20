# Ordnerrichtlinien

Eine Ordnerrichtlinie gibt jedem neu hinzukommenden Element eine eigene Lebensdauer. Ein typisches Beispiel ist der Downloadordner:

```powershell
obit watch C:\Users\you\Downloads 2w        # files that land in Downloads are buried two weeks later
obit watch --list                           # folder policies, their exemptions and quiet periods
obit watch --remove C:\Users\you\Downloads  # remove a policy and the lifespans it registered
```

## Wann die Richtlinie greift

- **Die Richtlinie muss zuerst genehmigt werden.** `obit watch` erstellt einen Vorschlag. Die Richtlinie gilt erst, wenn du ihn mit `obit approve` oder auf der Seite „For you“ im kleinen Fenster bestätigst.
- **Zehn Minuten ohne Änderungen.** Eine Datei wird erst begraben, wenn ihre Lebensdauer abgelaufen ist und sie seit mindestens zehn Minuten unverändert ist.
- **Für Unterordner gibt es eigene Vorschläge.** Läuft die Lebensdauer eines Unterordners ab, entsteht zunächst nur ein Vorschlag. Begraben wird er erst mit deiner Zustimmung.
- Wenn Sie eine Richtlinie entfernen, werden die von ihr registrierten Lebensdauern aufgehoben.

## Datumsanzeige im Explorer {#dates-in-explorer}

Im Downloadordner, auf dem Desktop und in Ordnern mit Richtlinie zeigt der Explorer registrierte Elemente etwa als „Name (Begräbnis am 13. November)“ an. Der tatsächliche Dateiname bleibt unverändert.

- Die Datumsangaben stammen aus der versteckten Datei `desktop.ini` im jeweiligen Ordner.
- Im Abschnitt `[explorer]` der Datei `config.toml` kannst du die Anzeige mit `enabled = false` abschalten oder mit `everywhere = true` überall einschalten.
- Der Explorer speichert Ordneransichten zwischen. Nach einer Verlängerung kann dasselbe Fenster noch bis zu einer Minute das alte Datum anzeigen.
