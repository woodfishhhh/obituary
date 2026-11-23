# Ordnerrichtlinien

Eine Ordnerrichtlinie gibt jedem neu hinzukommenden Element eine eigene Lebensdauer. Ein typisches Beispiel ist der Downloadordner:

```powershell
obit watch C:\Users\you\Downloads 2w        # files that land in Downloads are buried two weeks later
obit watch --list                           # folder policies, their exemptions and quiet periods
obit watch --remove C:\Users\you\Downloads  # remove a policy and the lifespans it registered
```

Unter macOS und Linux wird nach der Freigabe jeder direkte Eintrag einzeln erfasst. Die Frist beginnt standardmäßig beim ersten Erkennen. Der überwachte Ordner bleibt geschützt; für abgelaufene Unterordner gibt es nur Vorschläge. Temporäre Downloads werden übersprungen. Lassen sich Schreibzugriffe nicht ausschließen, bleibt der Eintrag auch nach zehn ruhigen Minuten liegen.

**Unter macOS und Linux werden abgelaufene Dateien derzeit nicht automatisch begraben; auch ein genehmigter Unterordner bleibt liegen.**

```sh
obit watch "$HOME/Downloads" 2w
```

## Wann die Richtlinie greift

- **Die Richtlinie muss zuerst genehmigt werden.** `obit watch` erstellt einen Vorschlag. Die Richtlinie gilt erst, wenn du ihn mit `obit approve` oder auf der Seite „For you“ im kleinen Fenster bestätigst.
- **Zehn Minuten ohne Änderungen.** Eine Datei wird erst begraben, wenn ihre Lebensdauer abgelaufen ist und sie seit mindestens zehn Minuten unverändert ist.
- **Für Unterordner gibt es eigene Vorschläge.** Läuft die Lebensdauer eines Unterordners ab, entsteht zunächst nur ein Vorschlag. Begraben wird er erst mit deiner Zustimmung.
- Wenn Sie eine Richtlinie entfernen, werden die von ihr registrierten Lebensdauern aufgehoben.

## Datumsanzeige im Explorer {#dates-in-explorer}

Im Downloadordner, auf dem Desktop und in Ordnern mit Richtlinie zeigt der Explorer registrierte Elemente etwa als „name (11-13 burial)“ an. Der tatsächliche Dateiname bleibt unverändert. Der Zusatz folgt der Sprache von Obituary: Bei vereinfachtem Chinesisch erscheint „name（11-13 下葬）“, sonst die englische Form.

- Die Datumsangaben stammen aus der versteckten Datei `desktop.ini` im jeweiligen Ordner.
- Im Abschnitt `[explorer]` der Datei `config.toml` kannst du die Anzeige mit `enabled = false` abschalten oder mit `everywhere = true` überall einschalten.
- Der Explorer speichert Ordneransichten zwischen. Nach einer Verlängerung kann dasselbe Fenster noch bis zu einer Minute das alte Datum anzeigen.
