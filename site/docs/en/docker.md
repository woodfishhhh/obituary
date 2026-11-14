# Docker containers

Containers started in an agent session can have a lifespan too.

```powershell
obit docker add web-1 3d       # register a container by name or ID; it expires in three days
obit docker list               # registered containers (--json works too)
obit resurrect <id>            # start a container that was stopped at expiry
```

## Containers only

Obituary manages containers only. Volumes hold data and removing one is permanent, so volumes, images and networks are never touched.

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

## Containers from an agent session are registered directly

When a session ends, Obituary looks in the background for the containers created during it: only those whose compose working directory or bind mounts are inside the project; others may be ones you started elsewhere and are left alone. Those found get a 3-day lifespan and you are told afterwards; after `obit spare <id>` the session does not register that container again. With `[proposals] mode = "ask"` they are proposed instead and tracked only after `obit approve`.

Hooks never call Docker; if Docker is not installed or not running, this is skipped quietly and `obit doctor` explains why.
