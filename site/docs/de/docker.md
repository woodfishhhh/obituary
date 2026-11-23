# Docker-Container und Volumes

Container, die in einer Agentensitzung gestartet werden, können auch eine Lebensdauer haben.

```powershell
obit docker add web-1 3d       # register a container by name or ID; it expires in three days
obit docker list               # registered containers (--json works too)
obit resurrect <id>            # start a container that was stopped at expiry
```

## Ein Volume vormerken oder sofort begraben

Volumes werden nie automatisch registriert. Obituary verarbeitet nur ausdrücklich registrierte oder begrabene Volumes und löscht weder Images noch Netzwerke.

Mit `obit mark --docker-volume cache 3d` (auch `add`) wird `cache` in drei Tagen begraben, mit `obit bury --docker-volume -- cache` sofort.

Docker muss Linux-Container verwenden. Unterstützt werden nur lokale Volumes mit dem Treiber `local` ohne zusätzliche Optionen. Fehlt das Hilfsimage beim ersten Einsatz, lädt Docker eine festgelegte Debian-Version herunter.

Der Friedhof braucht Platz für ein vollständiges Archiv. Erst nach erfolgreicher Archivierung und Prüfung wird das Volume gelöscht. Verweist noch ein Container darauf, selbst ein gestoppter, oder überschneidet sich der Pfad mit einem Bind-Mount, bleibt das Volume unangetastet.

`obit resurrect <ID>` legt das Volume unter seinem ursprünglichen Namen mit dem dauerhaften Label `io.obituary.restore` neu an. Ist der Name belegt, wird auch ein leeres Volume nicht überschrieben.

Docker-Ereignisse werden vor und nach dem Löschen geprüft. Lässt sich die Löschung nicht bestätigen, bleibt das geprüfte Archiv erhalten. Prüfe `obit doctor`, statt von einer erfolgreichen Bestattung auszugehen. Jede Warnung unter `docker-volume-recovery` nennt einen zu prüfenden Eintrag. Prüfe das Volume wie angegeben auf dem ursprünglichen Docker-Daemon. Nicht verbuchte Archive lassen sich weder mit `obit resurrect` noch mit `obit purge` bearbeiten.

Der Löschschutz erkennt `docker volume rm`, `docker volume remove`, `docker volume prune`, `docker compose down -v` (oder `--volumes`) und `docker system prune --volumes`. Nur bei einem einfachen `docker volume rm` mit ausdrücklichen Namen schlägt er `obit bury --docker-volume` vor.

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
