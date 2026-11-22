# Docker containers and volumes

Containers started in an agent session can have a lifespan too.

```powershell
obit docker add web-1 3d       # register a container by name or ID; it expires in three days
obit docker list               # registered containers (--json works too)
obit resurrect <id>            # start a container that was stopped at expiry
```

## Give a volume a lifespan or bury it now

Volumes are never registered automatically. Obituary only handles a volume you explicitly mark or bury; it does not remove images or networks.

Use `obit mark --docker-volume cache 3d` (`add` is an alias) to bury `cache` in three days, or `obit bury --docker-volume -- cache` to bury it now.

Docker must be in Linux container mode. Only local volumes using the `local` driver without extra options are supported. On first use, Docker pulls a pinned Debian helper image if it is not already available.

Leave room in the graveyard for a full archive. Obituary checks the archive before removing the volume; if archiving fails, the volume stays. A volume referenced even by a stopped container, or whose path overlaps a bind mount, is left alone.

`obit resurrect <id>` recreates the volume under its original name with a permanent `io.obituary.restore` label. If that name already exists, even on an empty volume, nothing is overwritten.

Docker events are checked before and after removal. If deletion cannot be confirmed, keep the verified archive and check `obit doctor`; do not assume the volume was buried successfully.

The delete guard recognizes `docker volume rm`, `docker volume remove`, `docker volume prune`, `docker compose down -v` (or `--volumes`) and `docker system prune --volumes`. It suggests `obit bury --docker-volume` only for a plain `docker volume rm` with explicit names; it does not guess what a prune would remove.

## Expiry means stopping

- After `docker stop` the container and its writable layer remain; `obit resurrect <id>` runs `docker start` and stops tracking its lifespan.
- Only after the retention period (`[graveyard] retention_days`, 7 days by default) is it removed with `docker rm`, never with `-v` or `-f`.
- If it is running by then (you or someone started it), it counts as resurrected and is kept.
- `obit purge` and `--all` also wait for the retention period for containers. Recycle mode does not apply to containers: they are still stopped first and removed after retention.

## Identified by full ID

- Names are only for display; before every stop, start or remove the full ID and creation time are checked again, and nothing happens if they differ.
- Obituary records the Docker context and daemon a container lives on and only ever acts on that daemon, so switching `docker context` cannot redirect it.
- A container you removed outside Obituary is recorded as removed externally and its record is cleared.
- A container started with `--rm` (AutoRemove) would be deleted by Docker when stopped, so Obituary refuses to register it.

## With automatic registration on, containers from a session are registered too

While automatic registration is off (the default) Obituary does not look for containers; `obit docker add` above registers one. With it on, when a session ends Obituary looks in the background for the containers created during it: only those whose compose working directory or bind mounts are inside the project; others may be ones you started elsewhere and are left alone. Those found get a 3-day lifespan and you are told afterwards; after `obit spare <id>` the session does not register that container again. When the session did not run in a Git project (one opened in your home folder, say), or several sessions claim the same container, it is unclear who started it, so it waits as a proposal for your approval. With `[proposals] mode = "ask"` they are proposed instead and tracked only after `obit approve`.

Each container gets one proposal at most. Hooks never call Docker; if Docker is not installed or not running, this is skipped quietly and `obit doctor` explains why. A container proposal cannot be approved while Docker is not running; `obit proposals` and the window's For you page say so, and it can be approved once Docker is up.
