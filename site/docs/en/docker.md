# Docker containers and volumes

Containers started in an agent session can have a lifespan too.

```powershell
obit docker add web-1 3d       # register a container by name or ID; it expires in three days
obit docker list               # registered containers (--json works too)
obit resurrect <id>            # start a container that was stopped at expiry
```

## Set a lifespan when starting a container

Temporary containers are easiest to forget when no lifespan is set up front. `docker run -d` prints the full container ID to standard output, so you can pass it straight to `obit docker add`:

```powershell
# PowerShell
$cid = docker run -d --name tmp-redis redis:7
if ($LASTEXITCODE -eq 0) { obit docker add $cid 4h }
```

```bash
# Bash / zsh (no name required; pass the returned ID directly)
cid=$(docker run -d redis:7) && obit docker add "$cid" 4h
```

`obit docker add <name-or-id> <lifespan>` accepts a container name, a short ID or the full 64-character ID, and has no `--at` or `--note` flags. It prints the ledger number and expiry time (`#<id> docker:<name> (<12-char-id>)  expires <time>`). Do not pass `--rm` to `docker run`: Docker removes an AutoRemove container as soon as it stops, so `obit docker add` refuses to register it. Before expiry, use the number from `obit docker list` or `obit list` with `obit extend <id> <lifespan>` or `obit spare <id>`.

## Inspect and recover stopped containers during retention

Once a container expires and `docker stop` runs, it leaves `obit docker list` (which shows only active tracked containers) and moves into the graveyard for `[graveyard] retention_days` (7 days by default). Use these commands to review or bring back stopped containers during retention:

```powershell
obit graveyard                 # list all restorable burials; containers show as docker:<name> (<12-char-id>) with purge countdown
obit graveyard --json          # machine-readable output; filter burials where kind == "container"
obit stats                     # total graveyard count and the next purge time
obit resurrect <id>            # start the stopped container by its numeric graveyard ID and stop tracking it
```

Containers added manually with `obit docker add` are not bound to a project directory, so `obit graveyard --here` omits them; only containers registered automatically or approved from a project proposal match `--here`. On desktop, the tray menu shows the graveyard count and up to the 10 most recent burials (click one to resurrect it), and the window's Graveyard page lets you search by name, check remaining retention time and resurrect selected entries. The background runner does not send proactive desktop notifications when a container stops or approaches purge, so check `obit graveyard` or the Graveyard page when you want to review stopped containers before the retention window closes.

## Give a volume a lifespan or bury it now

Volumes are never registered automatically. Obituary only handles a volume you explicitly mark or bury; it does not remove images or networks.

Use `obit mark --docker-volume cache 3d` (`add` is an alias) to bury `cache` in three days, or `obit bury --docker-volume -- cache` to bury it now.

Docker must be in Linux container mode. Only local volumes using the `local` driver without extra options are supported. On first use, Docker pulls a pinned Debian helper image if it is not already available.

Leave room in the graveyard for a full archive. Obituary checks the archive before removing the volume; if archiving fails, the volume stays. A volume referenced even by a stopped container, or whose path overlaps a bind mount, is left alone.

`obit resurrect <id>` recreates the volume under its original name with a permanent `io.obituary.restore` label. If that name already exists, even on an empty volume, nothing is overwritten.

Docker events are checked before and after removal. If deletion cannot be confirmed, keep the verified archive and check `obit doctor`; do not assume the volume was buried successfully. Each `docker-volume-recovery` warning identifies one entry to check. Inspect the volume on the original Docker daemon as directed. Neither `obit resurrect` nor `obit purge` accepts an uncommitted archive.

The delete guard recognizes `docker volume rm`, `docker volume remove`, `docker volume prune`, `docker compose down -v` (or `--volumes`) and `docker system prune --volumes`. It suggests `obit bury --docker-volume` only for a plain `docker volume rm` with explicit names; it does not guess what a prune would remove.

## Expiry means stopping

- After `docker stop` the container and its writable layer remain; `obit resurrect <id>` runs `docker start` and stops tracking its lifespan.
- Only after the retention period (`[graveyard] retention_days`, 7 days by default) is it removed with `docker rm`, never with `-v` or `-f`.
- If you start the container during retention, Obituary keeps it and stops tracking it, even if you stop it again. Older versions did not record the start time: Obituary leaves those containers alone, and you can still run `obit resurrect <id>`.
- `obit purge` and `--all` also wait for the retention period for containers. Recycle mode does not apply to containers: they are still stopped first and removed after retention.

## Identified by full ID

- Names are only for display; before every stop, start or remove the full ID and creation time are checked again, and nothing happens if they differ.
- Obituary records the Docker context and daemon a container lives on and only ever acts on that daemon, so switching `docker context` cannot redirect it.
- A container you removed outside Obituary is recorded as removed externally and its record is cleared.
- A container started with `--rm` (AutoRemove) would be deleted by Docker when stopped, so Obituary refuses to register it.

## With automatic registration on, containers from a session are registered too

While automatic registration is off (the default) Obituary does not look for containers; `obit docker add` above registers one. With it on, when a session ends Obituary looks in the background for the containers created during it: only those whose compose working directory or bind mounts are inside the project; others may be ones you started elsewhere and are left alone. Those found get a 3-day lifespan and you are told afterwards; after `obit spare <id>` the session does not register that container again. When the session did not run in a Git project (one opened in your home folder, say), or several sessions claim the same container, it is unclear who started it, so it waits as a proposal for your approval. With `[proposals] mode = "ask"` they are proposed instead and tracked only after `obit approve`.

Each container gets one proposal at most. Hooks never call Docker; if Docker is not installed or not running, this is skipped quietly and `obit doctor` explains why. A container proposal cannot be approved while Docker is not running; `obit proposals` and the window's For you page say so, and it can be approved once Docker is up.
