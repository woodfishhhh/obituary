# Contenedores de Docker

También puedes asignar una vida útil a los contenedores que se inicien durante una sesión del agente.

```powershell
obit docker add web-1 3d       # register a container by name or ID; it expires in three days
obit docker list               # registered containers (--json works too)
obit resurrect <id>            # start a container that was stopped at expiry
```

## Solo contenedores

Obituary solo gestiona contenedores. Los volúmenes almacenan datos y su eliminación es irreversible. Obituary nunca toca volúmenes, imágenes ni redes.

## Al caducar, el contenedor se detiene

- Tras `docker stop` se conservan tanto el contenedor como su capa de escritura. `obit resurrect <id>` ejecuta `docker start` y deja de hacer seguimiento de su vida útil.
- Solo después del período de retención (`[graveyard] retention_days`, 7 días por defecto) se elimina con `docker rm`, nunca con `-v` o `-f`.
- Si al final del plazo está en ejecución porque tú u otra persona lo habéis iniciado, se considera resucitado y se conserva.
- Ni `obit purge` ni `--all` permiten acortar el plazo de retención de los contenedores. El modo de papelera tampoco se aplica: primero se detienen y solo se eliminan pasado el plazo.

## Identificación por el ID completo

- El nombre solo sirve para mostrarlo. Antes de detener, iniciar o eliminar un contenedor, Obituary contrasta su ID completo y su hora de creación con los datos registrados. Si no coinciden, no actúa.
- Obituary registra el contexto de Docker y el demonio al que pertenece el contenedor. Solo actúa sobre ese demonio, incluso si cambias de `docker context`.
- Si eliminas el contenedor fuera de Obituary, se anota la eliminación externa y se borra la entrada de seguimiento.
- Docker elimina al detenerse los contenedores iniciados con `--rm` (AutoRemove). Por eso Obituary no permite registrarlos.

## Registro automático de contenedores de una sesión

Por defecto, el registro automático está desactivado: Obituary no busca contenedores, pero puedes registrar uno con `obit docker add`. Si lo activas, al finalizar la sesión busca en segundo plano los contenedores creados durante ella. Solo incluye aquellos cuyo directorio de trabajo de Compose o cuyos montajes de tipo bind están dentro del proyecto. Los demás podrían haberse iniciado en otro lugar y no se tocan. Asigna a los que encuentra una vida útil de tres días y te avisa después. Si cancelas el registro con `obit spare <id>`, esa sesión no volverá a registrar el contenedor. Si la sesión se ejecutó fuera de un proyecto Git (por ejemplo, en tu directorio personal), o si varias sesiones reclaman el mismo contenedor, no se sabe quién lo inició: Obituary presenta una propuesta y espera tu aprobación. Con `[proposals] mode = "ask"` también presenta propuestas y solo inicia el seguimiento tras `obit approve`.

Se crea como máximo una propuesta por contenedor. Los hooks nunca invocan Docker. Si no está instalado o no se está ejecutando, se omite el paso sin interrumpirte; `obit doctor` explica el motivo. Mientras Docker no esté en ejecución, no se pueden aprobar propuestas de contenedores. `obit proposals` y la página de decisiones de la ventana lo indican. Podrás aprobarlas cuando se inicie Docker.
