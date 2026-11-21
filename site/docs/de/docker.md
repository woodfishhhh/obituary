# Docker-Container

Container, die in einer Agentensitzung gestartet werden, können auch eine Lebensdauer haben.

```powershell
obit docker add web-1 3d       # register a container by name or ID; it expires in three days
obit docker list               # registered containers (--json works too)
obit resurrect <id>            # start a container that was stopped at expiry
```

## Nur für Container

Obituary verwaltet nur Container. Volumes enthalten Daten; das Löschen eines Volumes ist endgültig. Volumes, Images und Netzwerke bleiben daher unangetastet.

## Beim Ablauf wird der Container gestoppt

- Nach `docker stop` bleiben der Container und seine beschreibbare Schicht erhalten. `obit resurrect <id>` startet ihn mit `docker start` und beendet die Überwachung seiner Lebensdauer.
- Erst nach der Aufbewahrungsfrist (`[graveyard] retention_days`, standardmäßig 7 Tage) wird der Container mit `docker rm` entfernt, niemals mit `-v` oder `-f`.
- Läuft er zu diesem Zeitpunkt wieder, weil du oder jemand anders ihn gestartet hat, gilt er als wiederbelebt und bleibt erhalten.
- Auch `obit purge` und `--all` müssen bei Containern die Aufbewahrungsfrist abwarten. Der Papierkorbmodus gilt nicht für Container: Sie werden zuerst gestoppt und erst nach Ablauf der Frist entfernt.

## Identifikation anhand der vollständigen ID

- Der Name dient nur der Anzeige. Vor jedem Stoppen, Starten oder Entfernen vergleicht Obituary die vollständige ID und den Erstellungszeitpunkt mit dem gespeicherten Eintrag. Bei einer Abweichung passiert nichts.
- Obituary speichert den Docker-Kontext und den zugehörigen Daemon und führt Aktionen nur auf diesem Daemon aus. Ein Wechsel mit `docker context` kann sie nicht umleiten.
- Entfernst du einen Container außerhalb von Obituary, wird die externe Entfernung vermerkt und der Eintrag aus dem Register genommen.
- Docker löscht mit `--rm` (AutoRemove) gestartete Container beim Stoppen selbst. Diese Container lassen sich deshalb nicht registrieren.

## Container einer Sitzung automatisch registrieren

Standardmäßig ist die automatische Registrierung aus. Obituary sucht dann nicht nach Containern; mit `obit docker add` kannst du einzelne selbst registrieren. Ist sie eingeschaltet, sucht Obituary am Sitzungsende im Hintergrund nach Containern, die während der Sitzung erstellt wurden. Berücksichtigt werden nur Container, deren Compose-Arbeitsverzeichnis oder Bind-Mounts im Projekt liegen. Die anderen können anderswo gestartet worden sein und bleiben unangetastet. Gefundene Container bekommen drei Tage Lebensdauer, du wirst anschließend informiert. Nach `obit spare <id>` registriert dieselbe Sitzung den Container nicht erneut. Lief die Sitzung außerhalb eines Git-Projekts (etwa im Home-Verzeichnis), oder beanspruchen mehrere Sitzungen denselben Container, ist sein Ursprung unklar. Dann wird er nur vorgeschlagen und wartet auf deine Zustimmung. Mit `[proposals] mode = "ask"` werden ebenfalls nur Vorschläge erstellt; die Nachverfolgung beginnt erst nach `obit approve`.

Pro Container gibt es höchstens einen Vorschlag. Hooks rufen Docker nie auf. Ist Docker nicht installiert oder läuft es nicht, wird die Suche still übersprungen; `obit doctor` nennt den Grund. Solange Docker nicht läuft, können Containervorschläge nicht genehmigt werden. `obit proposals` und die Seite „For you“ im Fenster weisen darauf hin. Sobald Docker läuft, kannst du sie genehmigen.
