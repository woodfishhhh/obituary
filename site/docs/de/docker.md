# Docker-Container und Volumes

Container, die in einer Agentensitzung gestartet werden, können auch eine Lebensdauer haben.

```powershell
obit docker add web-1 3d       # register a container by name or ID; it expires in three days
obit docker list               # registered containers (--json works too)
obit resurrect <id>            # start a container that was stopped at expiry
```

## Lebensdauer direkt beim Starten setzen

Temporäre Container bleiben am ehesten liegen, wenn beim Start keine Lebensdauer festgelegt wird. Da `docker run -d` die vollständige Container-ID auf der Standardausgabe ausgibt, kannst du sie unmittelbar an `obit docker add` übergeben:

```powershell
# PowerShell
$cid = docker run -d --name tmp-redis redis:7
if ($LASTEXITCODE -eq 0) { obit docker add $cid 4h }
```

```bash
# Bash / zsh (no name required; pass the returned ID directly)
cid=$(docker run -d redis:7) && obit docker add "$cid" 4h
```

`obit docker add <name-or-id> <lifespan>` akzeptiert einen Containernamen, eine kurze ID oder die vollständige 64-stellige ID und kennt weder `--at` noch `--note`. Nach der Registrierung gibt der Befehl die Registernummer und den Ablaufzeitpunkt aus (`#<id> docker:<name> (<12-char-id>)  expires <time>`). Verwende bei `docker run` kein `--rm`: Docker löscht AutoRemove-Container sofort beim Stoppen, weshalb `obit docker add` die Registrierung ablehnt. Vor Ablauf kannst du die Nummer aus `obit docker list` oder `obit list` mit `obit extend <id> <lifespan>` oder `obit spare <id>` nutzen.

## Gestoppte Container während der Aufbewahrungsfrist prüfen und wiederbeleben

Sobald ein Container abläuft und mit `docker stop` angehalten wird, verschwindet er aus `obit docker list` (das nur aktiv verfolgte Container zeigt) und liegt für `[graveyard] retention_days` (standardmäßig 7 Tage) im Friedhof. Mit diesen Befehlen prüfst oder reaktivierst du gestoppte Container während der Frist:

```powershell
obit graveyard                 # list all restorable burials; containers show as docker:<name> (<12-char-id>) with purge countdown
obit graveyard --json          # machine-readable output; filter burials where kind == "container"
obit stats                     # total graveyard count and the next purge time
obit resurrect <id>            # start the stopped container by its numeric graveyard ID and stop tracking it
```

Manuell mit `obit docker add` registrierte Container sind keinem Projektverzeichnis zugeordnet und fehlen daher in `obit graveyard --here`; nur automatisch registrierte oder aus einem Projektvorschlag genehmigte Container passen zu `--here`. Auf dem Desktop zeigt das Tray-Menü die Anzahl der Einträge im Friedhof und bis zu 10 zuletzt begrabene Einträge (ein Klick belebt sie wieder), und auf der Friedhofsseite des Fensters kannst du nach Namen filtern, die verbleibende Frist prüfen und Einträge wiederbeleben. Der Hintergrunddienst sendet weder beim Stoppen eines Containers noch kurz vor der endgültigen Löschung eine proaktive Systembenachrichtigung; schau daher in `obit graveyard` oder auf die Friedhofsseite, bevor die Aufbewahrungsfrist endet.

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
- Startest du den Container während der Aufbewahrungsfrist, bleibt er erhalten und wird nicht weiter verfolgt, auch wenn du ihn wieder stoppst. Bei Containern aus älteren Versionen fehlt die Startzeit zum Vergleich. Obituary löscht sie nicht; mit `obit resurrect <id>` kannst du sie wieder starten.
- Auch `obit purge` und `--all` müssen bei Containern die Aufbewahrungsfrist abwarten. Der Papierkorbmodus gilt nicht für Container: Sie werden zuerst gestoppt und erst nach Ablauf der Frist entfernt.

## Identifikation anhand der vollständigen ID

- Der Name dient nur der Anzeige. Vor jedem Stoppen, Starten oder Entfernen vergleicht Obituary die vollständige ID und den Erstellungszeitpunkt mit dem gespeicherten Eintrag. Bei einer Abweichung passiert nichts.
- Obituary speichert den Docker-Kontext und den zugehörigen Daemon und führt Aktionen nur auf diesem Daemon aus. Ein Wechsel mit `docker context` kann sie nicht umleiten.
- Entfernst du einen Container außerhalb von Obituary, wird die externe Entfernung vermerkt und der Eintrag aus dem Register genommen.
- Docker löscht mit `--rm` (AutoRemove) gestartete Container beim Stoppen selbst. Diese Container lassen sich deshalb nicht registrieren.

## Container einer Sitzung automatisch registrieren

Standardmäßig ist die automatische Registrierung aus. Obituary sucht dann nicht nach Containern; mit `obit docker add` kannst du einzelne selbst registrieren. Ist sie eingeschaltet, sucht Obituary am Sitzungsende im Hintergrund nach Containern, die während der Sitzung erstellt wurden. Berücksichtigt werden nur Container, deren Compose-Arbeitsverzeichnis oder Bind-Mounts im Projekt liegen. Die anderen können anderswo gestartet worden sein und bleiben unangetastet. Gefundene Container bekommen drei Tage Lebensdauer, du wirst anschließend informiert. Nach `obit spare <id>` registriert dieselbe Sitzung den Container nicht erneut. Lief die Sitzung außerhalb eines Git-Projekts (etwa im Home-Verzeichnis), oder beanspruchen mehrere Sitzungen denselben Container, ist sein Ursprung unklar. Dann wird er nur vorgeschlagen und wartet auf deine Zustimmung. Mit `[proposals] mode = "ask"` werden ebenfalls nur Vorschläge erstellt; die Nachverfolgung beginnt erst nach `obit approve`.

Pro Container gibt es höchstens einen Vorschlag. Hooks rufen Docker nie auf. Ist Docker nicht installiert oder läuft es nicht, wird die Suche still übersprungen; `obit doctor` nennt den Grund. Solange Docker nicht läuft, können Containervorschläge nicht genehmigt werden. `obit proposals` und die Seite „For you“ im Fenster weisen darauf hin. Sobald Docker läuft, kannst du sie genehmigen.
