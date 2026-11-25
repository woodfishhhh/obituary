# Qué archivos se registran

## El agente y tú decidís; Obituary entierra a tiempo

El agente y tú decidís qué entra en Obituary (por defecto, `[proposals] mode = "off"`):

- **El agente registra sus propios resultados.** Asigna una vida útil con `obit mark <path> 2d` a los scripts de depuración, registros y capturas que escribe, crea espacio temporal con `obit mktemp 1d` y entierra con `obit bury` lo que no sabe si debe conservar. `obit init` explica este comportamiento al agente.
- **Tú añades elementos desde el menú contextual.** En el Explorador, Finder, Nautilus o Dolphin, haz clic derecho en archivos o carpetas (puedes seleccionar varios), elige «Add to Obituary» y después 1, 7 o 30 días, o «Bury now (recoverable)». Consulta [Ventana, bandeja y menú contextual](./desktop).
- **Las políticas de carpetas** asignan una vida útil a carpetas como Descargas. Consulta [Políticas de carpetas](./folder-policies).

Todo lo registrado se entierra cuando caduca y puede resucitarse tal como estaba durante siete días. Nada más se registra por sí solo.

- **Limpieza al final de la sesión.** Cuando un agente de Claude Code o Codex va a detenerse, Obituary enumera los archivos que creó en esa sesión y aún no se han clasificado. Le pide asignar una vida útil o enterrar scripts, registros y capturas, conservando los entregables. Ocurre como máximo una vez por sesión y no muestra nada si no hay elementos; los demás agentes reciben el aviso al iniciar la siguiente sesión. `[hooks] session_cleanup = false` lo desactiva.
- **Al actualizar desde una versión anterior**, este ajuste se desactiva una vez y las dependencias, salidas de compilación y cachés registrados automáticamente antes quedan en su lugar, sin vida útil.

## Activa el registro automático cuando lo desees

El proceso residente registra los archivos después de la escritura, por lo que un archivo nuevo puede tardar un momento en aparecer en `obit list`.

Actívalo desde Settings o For you, o define `[proposals] mode` en `config.toml`.

- `"tiered"` registra directamente los archivos creados por el agente en esta sesión y los [contenedores Docker](./docker) iniciados para el proyecto. Un archivo solo entra directamente si el hook observó la escritura, no estaba en la instantánea inicial y su fecha de creación es posterior al inicio de la sesión. Se avisa al agente y aparece en `obit proposals` y en «Handled automatically» de For you. Los archivos anteriores a la sesión o cuya escritura no observó el hook solo generan una propuesta. `obit approve <id>` registra lo propuesto para el proyecto; desde entonces sus coincidencias de reglas pueden registrarse directamente. `obit reject <id>` rechaza el registro automático en este proyecto.
- `"auto"` registra directamente todo lo que encuentre el registro automático y te informa después.
- `"ask"` convierte todo en una propuesta pendiente de aprobación.

```powershell
obit proposals           # proposals waiting for you, and what was registered without asking
obit approve 3           # approve: register what it lists
obit reject 3            # reject
obit spare 12            # undo an automatic registration: it stays where it is, without a lifespan
obit extend 12 1w        # keep it longer
```

- **Dependencias, salidas de compilación y cachés** (`node_modules`, `dist`, `.turbo`, `__pycache__` y similares) nunca se registran ni se proponen automáticamente. Para limpiarlos, regístralos tú con `obit mark`.
- **Los elementos grandes** que no creó el agente también se proponen si superan 1 GB o contienen más de 200 archivos.
- **Diferencia de sesión.** Se toma una instantánea al comenzar la sesión, se guarda en el directorio de datos y no se añade nada al repositorio. Al terminar se compara; las coincidencias de reglas siguen el modo y los archivos nuevos sin regla no se registran ni se proponen.
- **Después de cambiar el modo**, el proceso en segundo plano vuelve a clasificar las propuestas pendientes.
- **Git se consulta de nuevo antes del entierro.** Lo registrado por regla o propuesta no se entierra si ya está en un commit cuando caduca.
- **Las propuestas caducan solas.** Los elementos confirmados o eliminados se quitan, y una propuesta vacía se retira, queda en el registro de auditoría y no cuenta como rechazo.

Los volúmenes nunca se registran automáticamente. Obituary solo trata los que registres o entierres expresamente y no elimina imágenes ni redes.

## Reglamento

```powershell
obit rules                       # the rules in effect here (built-in plus a trusted .obitrules)
obit rules check debug.log out/  # preview what would be registered and by which rule (changes nothing)
obit rules apply debug.log       # register by rule
```

`obit rules apply` registra según las reglas en cualquier momento; con el registro automático activo, los hooks también las usan. Las reglas incluidas cubren residuos habituales como `.playwright-mcp/`, `test-results/` y `*.log`. Las copias de seguridad `*.bak`, `*.bak.*`, `*.orig`, `*.rej`, `*.old`, `*.backup` y `*~` se conservan siete días, pero `LOG.old` de LevelDB se deja intacto. El archivo `.obitrules` en la raíz usa sintaxis de `.gitignore` y una vida útil después de cada patrón.

```
output/**        7d      # every item under output gets its own lifespan
!output/keep/**          # never touch (negation wins, even over registering a parent folder)
test-results/    off     # turn off a built-in rule
```

A diferencia de `.gitignore`, las negaciones tienen prioridad sobre la última coincidencia. En Windows no se distingue entre mayúsculas y minúsculas. Un patrón inválido, como `[` sin cerrar, produce un error con archivo y línea y detiene el registro automático del proyecto.

Las unidades son `s`, `m`, `h`, `d` y `w`; se pueden combinar, por ejemplo `1d12h`.

## Reglas estrictas para el registro automático

- Solo se registran elementos sin seguimiento dentro de un árbol de trabajo Git.
- Una carpeta que contenga aunque sea un archivo seguido nunca se toca.
- Si Git no responde, se omite.
- Nunca se acorta una vida útil existente.
- Un proyecto rechazado nunca se registra directamente. Las políticas de carpetas y la confianza en `.obitrules` siempre esperan tu decisión.

## Las reglas del proyecto deben ser confiables primero

El `.obitrules` del repositorio no tiene efecto hasta que lo revises y ejecutes `obit trust`. Así, un repositorio ajeno no puede usar reglas para tomar archivos privados. Se guarda el SHA-256 del archivo; cualquier cambio exige una nueva revisión. Las sesiones no interactivas necesitan `--yes` y en CI puedes establecer `OBIT_TRUST_PROJECT_RULES=1`.

## Una carpeta temporal que se entierra a sí misma

```powershell
$dir = obit mktemp 1d            # .obit/tmp/ in a project (not in Git), otherwise the system temp folder
```

En bash: `dir=$(obit mktemp 1d)`.
