# Comandos

Sustituye `<id>` por el número que aparece al ejecutar `obit add` o `obit bury`, o en la lista de `obit list` o `obit graveyard`. Las unidades de tiempo `s`, `m`, `h`, `d` y `w` se pueden combinar, por ejemplo `1d12h`.

## Vida útil

```powershell
obit add debug.log 2h                       # bury it in 2 hours (alias: mark)
obit add output\shots 1d12h                 # folders work too
obit add report.zip --at "2026-12-01 18:00" # at a given moment
obit list                                   # what is left and for how long
obit extend <id> 1d                         # one more day
obit spare <id>                             # stop tracking, keep the file (alias: cancel)
obit reap                                   # bury everything that is due now (alias: sweep; --dry-run previews)
```

## El cementerio

```powershell
obit bury old-build                         # bury right away
obit graveyard                              # what is in the graveyard and when it will be purged (--here: this folder only)
obit resurrect <id>                         # put it back; if the name is taken, it comes back renamed, never overwriting
obit resurrect --last                       # undo the last deletion: everything one obit bury / obit rm buried comes back
obit purge                                  # purge what is past retention (--all for everything, after confirming; --dry-run previews)
obit purge <id>                             # purge this grave now (permanent; asks first, --yes outside a terminal)
```

En `resurrect` y `purge`, un número sin ruta (`5` o `#5`) siempre es un identificador. Si no existe, el comando da un error; nunca lo interpreta como nombre de archivo. Para un archivo de nombre numérico, escribe `./5` o una ruta absoluta.

## Comandos de eliminación recuperable

```powershell
obit rm -rf build            # rm's common options; every path is buried
obit rmdir empty-dir
obit git-clean -fdx
obit remove-item .\build     # one literal path only
```

## Reglas y propuestas

```powershell
obit rules                       # the rules in effect here
obit rules check debug.log out/  # preview what would be registered and by which rule
obit rules apply debug.log       # register by rule
obit trust                       # trust this project's .obitrules (--yes when non-interactive)
obit proposals                   # proposals waiting for you, and what was registered without asking
obit approve 3                   # approve
obit reject 3                    # reject
$dir = obit mktemp 1d            # a temporary folder that buries itself
```

## Políticas de carpeta y Docker

```powershell
obit watch C:\Users\you\Downloads 2w
obit watch --list
obit watch --remove C:\Users\you\Downloads
obit docker add web-1 3d
obit docker list
```

Con `obit mark --docker-volume cache 3d` (también `add`), `cache` se enterrará dentro de tres días. Para enterrarlo ahora, usa `obit bury --docker-volume -- cache`. [Contenedores y volúmenes de Docker](./docker)

## Integración y tareas en segundo plano

```powershell
obit init                       # Claude Code hooks and Skill, context menu, Start menu and desktop shortcuts; the tray starts at login
obit init -g --agent codex      # Codex
obit init -g --guard-rm         # delete guard
obit init -g --mcp              # add the MCP server
obit init --no-context-menu --no-shortcuts --no-skill   # skip the context menu, shortcuts or Skill
obit init -g --uninstall --no-context-menu   # on uninstall keep the context menu and remove the rest; --no-shortcuts and --no-skill work the same way
obit init -g --show             # what is installed
obit daemon enable              # tray at login plus the fallback scheduled task
obit daemon status
obit daemon disable
```

Con la supervisión de escritorio activada, el proceso de la bandeja atiende los vencimientos y una tarea independiente comprueba los pendientes cada hora. Sin supervisión de la bandeja para ese directorio de datos, la tarea se ejecuta cada cinco minutos.

## Consultas y comprobaciones

```powershell
obit doctor              # health check: short summary, full HTML report (--open, --verbose, --json)
obit stats               # eulogy: how much was buried, space freed, resurrections (alias: eulogy)
obit log                 # the full record of everything done
obit config              # show the configuration; --create writes a commented default
obit config telemetry off # disable usage reporting and clear the local installation ID; update checks are separate
obit hook audit          # with OBIT_HOOK_AUDIT=1: what each hook received and did
obit uninstall           # remove everything Obituary set up (try --dry-run first)
```

Los comandos que devuelven información o resultados (`list`, `graveyard`, `proposals`, `stats`, `mark`, `bury`, `resurrect` y otros) admiten `--json` para scripts y agentes. Los que envuelven un comando de eliminación, como `obit rm`, conservan el formato de salida original y no admiten `--json`.

## Variables de entorno

| Variable | Efecto |
|---|---|
| `OBIT_DISABLED=1` | Desactiva temporalmente los hooks |
| `OBIT_TELEMETRY=0`, `DO_NOT_TRACK=1` | Desactiva el envío de estadísticas de uso para este proceso; en CI no se envían |
| `OBIT_GUARD_RM=0` | Desactiva temporalmente el guardia de eliminación |
| `OBIT_HOOK_AUDIT=1` | Registra qué recibió e hizo cada hook |
| `OBIT_TRUST_PROJECT_RULES=1` | En CI, confía en el archivo `.obitrules` del proyecto |
| `OBIT_HOME`, `OBIT_CONFIG_DIR`, `OBIT_USER_HOME` | Cambian las ubicaciones del directorio de datos, el de configuración y el directorio personal del usuario |



## Parámetros compatibles de los envoltorios

- git-clean acepta las opciones de Git clean -d, -f, -n, -q, -x, -X, -e pattern, --exclude pattern y --, además de --dry-run, --force y --quiet.
- rm acepta -r, -R, -f, -d, -v, --recursive, --force, --dir, --verbose, --preserve-root y --, seguidos de rutas.
- rmdir solo acepta -v o --verbose y rutas; rmdir -p se rechaza.
- En Windows, remove-item solo acepta -Recurse, -Force, -Path o -LiteralPath y una única ruta literal. Para comodines en Bash usa obit rm.

<!-- generated-command-reference:start -->
## Referencia de parámetros

### `obit`

Obituary (讣告), la papelera de los agentes de IA: da una vida útil a los archivos y, cuando se agota, los entierra en un cementerio del que se pueden resucitar

Uso: `obit [command]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de obit |
| `--version` | `-v` | `false` | Versión de obit |

### `obit add`

Asignar una vida útil a rutas (si se vuelve a registrar una ruta con seguimiento, se reinicia su caducidad)

Alias: `add`, `mark`
Uso: `obit add <path>... <duration> | add <path>... --at <time> [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--at` | ninguno | `""` | Caducar en una hora local concreta: "2006-01-02 15:04", "2006-01-02" o "15:04" |
| `--docker-volume` | ninguno | `false` | Interpretar los argumentos como nombres de volúmenes de Docker |
| `--force` | ninguno | `false` | Permitir carpetas que contienen un repositorio .git |
| `--help` | `-h` | `false` | Ayuda de add |
| `--json` | ninguno | `false` | Imprimir JSON |
| `--no-daemon` | ninguno | `false` | No iniciar el demonio en segundo plano |
| `--note` | ninguno | `""` | Nota de texto libre que se muestra en la lista |

Ejemplos:

```text
obit add debug.log 2h
obit add output\shots 1d12h
obit add report.zip --at "2026-12-01 18:00"
```

### `obit approve`

Aprobar propuestas: registrar lo que enumeran

Uso: `obit approve <proposal id>... [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de approve |
| `--json` | ninguno | `false` | Imprimir JSON |

### `obit bury`

Mover rutas al cementerio ahora (se restauran con obit resurrect)

Uso: `obit bury <path>... [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--docker-volume` | ninguno | `false` | Interpretar los argumentos como nombres de volúmenes de Docker |
| `--force` | ninguno | `false` | Permitir carpetas que contienen un repositorio .git |
| `--help` | `-h` | `false` | Ayuda de bury |
| `--json` | ninguno | `false` | Imprimir JSON |
| `--note` | ninguno | `""` | Nota de texto libre que se guarda con el entierro |

### `obit cancel`

Perdonar rutas o ID de contenedores: dejar de hacerles seguimiento y conservar los objetos

Alias: `cancel`, `spare`
Uso: `obit cancel <id|path>... [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de cancel |
| `--json` | ninguno | `false` | Imprimir JSON |

### `obit completion`

Generar el script de autocompletado de obit para el shell indicado.

Uso: `obit completion [command]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de completion |

### `obit completion bash`

Generar el script de autocompletado para el shell bash.

Uso: `obit completion bash`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de bash |
| `--no-descriptions` | ninguno | `false` | Desactivar las descripciones del autocompletado |

### `obit completion fish`

Generar el script de autocompletado para el shell fish.

Uso: `obit completion fish [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de fish |
| `--no-descriptions` | ninguno | `false` | Desactivar las descripciones del autocompletado |

### `obit completion powershell`

Generar el script de autocompletado para powershell.

Uso: `obit completion powershell [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de powershell |
| `--no-descriptions` | ninguno | `false` | Desactivar las descripciones del autocompletado |

### `obit completion zsh`

Generar el script de autocompletado para el shell zsh.

Uso: `obit completion zsh [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de zsh |
| `--no-descriptions` | ninguno | `false` | Desactivar las descripciones del autocompletado |

### `obit config`

Mostrar el archivo de configuración y los ajustes efectivos (--create escribe los valores predeterminados)

Uso: `obit config [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--create` | ninguno | `false` | Escribir un archivo de configuración con los valores predeterminados |
| `--help` | `-h` | `false` | Ayuda de config |
| `--language` | ninguno | `""` | Establecer el idioma: auto, en o zh-CN |

### `obit config telemetry`

Estadísticas de uso: un ID de instalación aleatorio, la versión, el sistema operativo, la arquitectura, el idioma de la interfaz y el canal de instalación sirven para medir el número de instalaciones y el uso de cada versión. No se envían archivos, rutas ni comandos, y no se almacenan direcciones IP ni agentes de usuario. Se desactivan con obit config telemetry off o OBIT_TELEMETRY=0.

Uso: `obit config telemetry on|off [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de telemetry |

### `obit daemon`

Controlar el demonio en segundo plano (obitd)

Uso: `obit daemon [command]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de daemon |

### `obit daemon disable`

Eliminar la tarea programada y el demonio residente

Uso: `obit daemon disable [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--dry-run` | ninguno | `false` | Solo mostrar los cambios |
| `--help` | `-h` | `false` | Ayuda de disable |

### `obit daemon enable`

Activar la tarea de usuario (cada hora con supervisión de la bandeja; cada 5 minutos sin ella) y, en Windows, la bandeja al iniciar sesión

Uso: `obit daemon enable [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--dry-run` | ninguno | `false` | Solo mostrar los cambios |
| `--help` | `-h` | `false` | Ayuda de enable |
| `--resident` | ninguno | `false` | Activar la bandeja sin modificar la tarea programada de respaldo |

### `obit daemon start`

Iniciar el demonio ahora

Uso: `obit daemon start [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de start |

### `obit daemon status`

Mostrar el estado del demonio y la próxima caducidad

Uso: `obit daemon status [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de status |
| `--json` | ninguno | `false` | Imprimir JSON |

### `obit daemon stop`

Detener el demonio

Uso: `obit daemon stop [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de stop |

### `obit docker`

Hacer seguimiento de los contenedores de Docker por su ID completo inmutable y su fecha de creación. Al caducar, el contenedor se detiene y se conserva su capa de escritura. Al resucitarlo, se inicia. Una vez transcurridos los días de [graveyard] retention_days, la eliminación definitiva borra los contenedores detenidos sin borrar los volúmenes; esto también vale en modo papelera. Las imágenes, los volúmenes y las redes se conservan.

Uso: `obit docker [command]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de docker |

### `obit docker add`

Registrar un contenedor de Docker; al caducar se detiene

Uso: `obit docker add <container name|ID> <lifespan> [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de add |
| `--json` | ninguno | `false` | Imprimir JSON |

### `obit docker list`

Listar los contenedores de Docker registrados

Uso: `obit docker list [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de list |
| `--json` | ninguno | `false` | Imprimir JSON |

### `obit doctor`

Comprobar la instalación, las integraciones y los datos; solo escribe su propio informe

Uso: `obit doctor [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de doctor |
| `--json` | ninguno | `false` | Imprimir JSON |
| `--open` | ninguno | `false` | Abrir el informe HTML completo en el navegador predeterminado |
| `--verbose` | ninguno | `false` | Imprimir todas las comprobaciones en texto completo, con los detalles de las capacidades del agente |

### `obit extend`

Ampliar el plazo, retrasando la caducidad (si ya ha vencido, se cuenta desde ahora)

Uso: `obit extend <id|path>... <duration> [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de extend |
| `--json` | ninguno | `false` | Imprimir JSON |

Ejemplos:

```text
obit extend 3 1d
obit extend debug.log 30m
```

### `obit git-clean`

Enterrar las rutas seleccionadas por la simulación de git clean

Uso: `obit git-clean [options] [--] [pathspec]... [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de git-clean |

### `obit graveyard`

Listar lo que hay en el cementerio y aún se puede resucitar

Alias: `graveyard`, `graves`
Uso: `obit graveyard [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de graveyard |
| `--here` | ninguno | `false` | Mostrar solo las rutas originales situadas en el directorio actual y en sus subdirectorios |
| `--json` | ninguno | `false` | Imprimir JSON |

### `obit help`

Mostrar la ayuda de cualquier comando de la aplicación.

Uso: `obit help [command] [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de help |

### `obit hook`

Puntos de entrada de los hooks de agente (los llama el agente, no las personas)

Uso: `obit hook [command]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de hook |

### `obit hook antigravity`

Procesar un evento de hook de antigravity desde stdin; ante un error se permite la acción

Uso: `obit hook antigravity [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de antigravity |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook audit`

Mostrar los registros de auditoría de los hooks y de la guardia de eliminación

Uso: `obit hook audit [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de audit |
| `--lines` | `-n` | `40` | Número de líneas por origen |

### `obit hook claude`

Procesar un evento de hook de claude desde stdin; ante un error se permite la acción

Uso: `obit hook claude [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de claude |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook cline`

Procesar un evento de hook de cline desde stdin; ante un error se permite la acción

Uso: `obit hook cline [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de cline |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook codebuddy`

Procesar un evento de hook de codebuddy desde stdin; ante un error se permite la acción

Uso: `obit hook codebuddy [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de codebuddy |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook codebuddy-ide`

Procesar un evento de hook de codebuddy-ide desde stdin; ante un error se permite la acción

Uso: `obit hook codebuddy-ide [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de codebuddy-ide |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook codex`

Procesar un evento de hook de codex desde stdin; ante un error se permite la acción

Uso: `obit hook codex [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de codex |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook copilot`

Procesar un evento de hook de copilot desde stdin; ante un error se permite la acción

Uso: `obit hook copilot [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de copilot |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook cursor`

Procesar un evento de hook de cursor desde stdin; ante un error se permite la acción

Uso: `obit hook cursor [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de cursor |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook deepseek-harness`

Procesar un evento de hook de deepseek-harness desde stdin; ante un error se permite la acción

Uso: `obit hook deepseek-harness [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de deepseek-harness |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook droid`

Procesar un evento de hook de droid desde stdin; ante un error se permite la acción

Uso: `obit hook droid [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de droid |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook gemini`

Procesar un evento de hook de gemini desde stdin; ante un error se permite la acción

Uso: `obit hook gemini [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de gemini |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook hermes`

Procesar un evento de hook de hermes desde stdin; ante un error se permite la acción

Uso: `obit hook hermes [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de hermes |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook kilocode`

Procesar un evento de hook de kilocode desde stdin; ante un error se permite la acción

Uso: `obit hook kilocode [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de kilocode |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook kimi`

Procesar un evento de hook de kimi desde stdin; ante un error se permite la acción

Uso: `obit hook kimi [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de kimi |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook lingma`

Procesar un evento de hook de lingma desde stdin; ante un error se permite la acción

Uso: `obit hook lingma [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de lingma |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook minimax-code`

Procesar un evento de hook de minimax-code desde stdin; ante un error se permite la acción

Uso: `obit hook minimax-code [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de minimax-code |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook omp`

Procesar un evento de hook de omp desde stdin; ante un error se permite la acción

Uso: `obit hook omp [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de omp |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook openclaw`

Procesar un evento de hook de openclaw desde stdin; ante un error se permite la acción

Uso: `obit hook openclaw [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de openclaw |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook opencode`

Procesar un evento de hook de opencode desde stdin; ante un error se permite la acción

Uso: `obit hook opencode [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de opencode |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook pi`

Procesar un evento de hook de pi desde stdin; ante un error se permite la acción

Uso: `obit hook pi [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de pi |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook qoder`

Procesar un evento de hook de qoder desde stdin; ante un error se permite la acción

Uso: `obit hook qoder [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de qoder |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook qoder-cn`

Procesar un evento de hook de qoder-cn desde stdin; ante un error se permite la acción

Uso: `obit hook qoder-cn [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de qoder-cn |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook trae`

Procesar un evento de hook de trae desde stdin; ante un error se permite la acción

Uso: `obit hook trae [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de trae |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook trae-cn`

Procesar un evento de hook de trae-cn desde stdin; ante un error se permite la acción

Uso: `obit hook trae-cn [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de trae-cn |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook vibe`

Procesar un evento de hook de vibe desde stdin; ante un error se permite la acción

Uso: `obit hook vibe [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de vibe |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook zcode`

Procesar un evento de hook de zcode desde stdin; ante un error se permite la acción

Uso: `obit hook zcode [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | Nombre del evento del host cuando no figura en el payload |
| `--help` | `-h` | `false` | Ayuda de zcode |
| `--notice-only` | ninguno | `false` | Entregar contexto informativo sin la guardia de eliminación |

### `obit hook finish`

Calcular los archivos sobrantes de una sesión (los inician los hooks en segundo plano)

Uso: `obit hook finish [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de finish |
| `--project` | ninguno | `""` | Raíz del proyecto |
| `--session` | ninguno | `""` | ID de la sesión del agente |

### `obit init`

Instalar (o desinstalar con --uninstall) los hooks y las instrucciones de Obituary para un agente

Uso: `obit init [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--agent` | ninguno | `[claude]` | Agentes con los que integrar (repetir la opción o separarlos con comas; predeterminado: claude) |
| `--context-menu` | ninguno | `false` | Instalar los menús contextuales del gestor de archivos (activado por defecto) |
| `--dry-run` | ninguno | `false` | Solo mostrar los cambios |
| `--format` | ninguno | `"text"` | Formato de salida de --show: text o markdown (cobertura de agentes) |
| `--global` | `-g` | `false` | Para todo el usuario, en lugar de solo este proyecto |
| `--guard-rm` | ninguno | `false` | Denegar los comandos de eliminación admitidos y sugerir alternativas de obit restaurables |
| `--help` | `-h` | `false` | Ayuda de init |
| `--mcp` | ninguno | `false` | Registrar el servidor MCP stdio de Obituary |
| `--no-context-menu` | ninguno | `false` | Omitir los menús contextuales del gestor de archivos |
| `--no-shortcuts` | ninguno | `false` | Omitir los accesos directos del menú Inicio y del escritorio |
| `--no-skill` | ninguno | `false` | Dejar sin cambios la Agent Skill de Obituary |
| `--show` | ninguno | `false` | Mostrar lo que está instalado |
| `--skill` | ninguno | `false` | Instalar la Agent Skill de Obituary (activado por defecto) |
| `--uninstall` | ninguno | `false` | Quitar lo que Obituary ha añadido |
| `--verbose` | ninguno | `false` | Con --show: enumerar los detalles de las capacidades de cada agente |
| `--yes` | ninguno | `false` | Aplicar sin preguntar |

Ejemplos:

```text
obit init -g  # Claude Code, para todos los proyectos (~/.claude)
obit init  # Claude Code, solo este proyecto (.claude/)
obit init -g --agent codex  # Codex (~/.codex/hooks.json + AGENTS.md)
obit init -g --uninstall  # Quitar solo lo que Obituary ha añadido
obit init -g --show  # Lo que está instalado
obit init --show --format markdown > docs/agents.md
```

### `obit list`

Mostrar las rutas con seguimiento y los contenedores de Docker, empezando por la caducidad más próxima

Alias: `list`, `ls`
Uso: `obit list [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de list |
| `--json` | ninguno | `false` | Imprimir JSON |

### `obit log`

Mostrar el registro de auditoría de todo lo que ha hecho Obituary

Uso: `obit log [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de log |
| `--json` | ninguno | `false` | Imprimir JSON |
| `--lines` | `-n` | `20` | Número de eventos |

### `obit mcp`

Ofrecer las herramientas de Obituary mediante MCP stdio

Uso: `obit mcp [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de mcp |

### `obit mktemp`

Crear un directorio temporal que se entierra solo e imprimir su ruta

Uso: `obit mktemp <lifespan> [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de mktemp |
| `--json` | ninguno | `false` | Imprimir JSON |
| `--name` | ninguno | `"tmp"` | Prefijo del nombre del directorio |
| `--session` | ninguno | `""` | Sesión de agente a la que pertenece el directorio |

Ejemplos:

```text
dir=$(obit mktemp 1d)
obit mktemp 2h --name screenshots
```

### `obit proposals`

Registros pendientes de tu aprobación y los hechos sin preguntar

Uso: `obit proposals [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de proposals |
| `--json` | ninguno | `false` | Imprimir JSON |

### `obit purge`

Eliminar los entierros de archivos seleccionados o con la retención vencida. --all y las referencias explícitas se saltan la retención, por lo que piden confirmación antes; fuera de un terminal exigen --yes. Los contenedores de Docker requieren que haya transcurrido [graveyard] retention_days, incluso con --all o con un ID explícito; los contenedores en ejecución se consideran resucitados. La eliminación de Docker conserva los volúmenes y nunca fuerza la eliminación. El tratamiento de los contenedores es idéntico en modo papelera.

Alias: `purge`, `cremate`
Uso: `obit purge [id|original path]... [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--all` | ninguno | `false` | Eliminar definitivamente todos los entierros, no solo los que han superado su retención |
| `--dry-run` | ninguno | `false` | Mostrar solo lo que ocurriría |
| `--help` | `-h` | `false` | Ayuda de purge |
| `--json` | ninguno | `false` | Imprimir JSON |
| `--yes` | ninguno | `false` | Eliminar definitivamente --all o los entierros seleccionados sin preguntar (agentes: preguntar antes al usuario) |

### `obit reject`

Rechazar propuestas

Uso: `obit reject <proposal id>... [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de reject |
| `--json` | ninguno | `false` | Imprimir JSON |

### `obit remove-item`

Enterrar elementos del sistema de archivos con un subconjunto limitado de parámetros de PowerShell.

Uso: `obit remove-item [-Recurse] [-Force] [-Path|-LiteralPath] <path> [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de remove-item |

### `obit resurrect`

Restaurar archivos sin sobrescribir; iniciar por ID los contenedores de Docker enterrados

Alias: `resurrect`, `raise`
Uso: `obit resurrect <id|original path>... | --last [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de resurrect |
| `--json` | ninguno | `false` | Imprimir JSON |
| `--last` | ninguno | `false` | Restaurar el último lote de eliminaciones de la sesión actual o del proyecto Git |

### `obit rm`

Mover rutas al cementerio ahora (se restauran con obit resurrect)

Uso: `obit rm [options] <path>... [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de rm |

### `obit rmdir`

Mover rutas al cementerio ahora (se restauran con obit resurrect)

Uso: `obit rmdir [options] <path>... [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de rmdir |

### `obit rules`

Mostrar las reglas vigentes aquí: las integradas más el .obitrules del proyecto cuando se confía en él

Uso: `obit rules [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de rules |
| `--json` | ninguno | `false` | Imprimir JSON |

### `obit rules apply`

Registrar rutas según las reglas (los archivos con seguimiento de Git nunca se registran)

Uso: `obit rules apply <path>... [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de apply |
| `--json` | ninguno | `false` | Imprimir JSON |
| `--session` | ninguno | `""` | Sesión de agente de la que proceden las rutas |

### `obit rules check`

Mostrar qué registrarían las reglas para las rutas (no cambia nada)

Uso: `obit rules check <path>... [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de check |
| `--json` | ninguno | `false` | Imprimir JSON |
| `--session` | ninguno | `""` | Sesión de agente de la que proceden las rutas |

### `obit stats`

Lo que ha hecho Obituary: elementos con seguimiento, entierros, espacio recuperado y resurrecciones

Alias: `stats`, `eulogy`
Uso: `obit stats [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de stats |
| `--json` | ninguno | `false` | Imprimir JSON |

### `obit sweep`

Recoger ahora: enterrar todo lo que ha vencido sin esperar al proceso en segundo plano

Alias: `sweep`, `reap`
Uso: `obit sweep [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--dry-run` | ninguno | `false` | Mostrar solo lo que ocurriría |
| `--help` | `-h` | `false` | Ayuda de sweep |
| `--json` | ninguno | `false` | Imprimir JSON |

### `obit trust`

Revisar y confiar en el .obitrules del proyecto (cada edición requiere una nueva revisión)

Uso: `obit trust [rules file] [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de trust |
| `--yes` | ninguno | `false` | Confiar sin preguntar (solo después de revisar las reglas) |

### `obit uninstall`

Quitar las integraciones y los servicios en segundo plano de Obituary; los datos se conservan por defecto

Uso: `obit uninstall [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--dry-run` | ninguno | `false` | Solo imprimir el plan de desinstalación; no cambiar archivos ni servicios |
| `--help` | `-h` | `false` | Ayuda de uninstall |
| `--purge-data` | ninguno | `false` | Eliminar definitivamente las tumbas listadas y quitar los datos propios de Obituary; conservar los archivos desconocidos |
| `--yes` | ninguno | `false` | Confirmar el plan (y la eliminación definitiva de datos solo si también se indica --purge-data) |

### `obit untrust`

Dejar de aplicar el .obitrules del proyecto

Uso: `obit untrust [rules file] [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ayuda de untrust |

### `obit update`

Comprobar o instalar una actualización estable

Uso: `obit update [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--apply` | ninguno | `false` | Descargar, verificar e instalar la versión estable más reciente guardada en caché (una instalación manual solo sustituye los programas propios de Obituary) |
| `--check` | ninguno | `false` | Comprobar GitHub ahora, incluso si las comprobaciones automáticas están desactivadas |
| `--help` | `-h` | `false` | Ayuda de update |
| `--relaunch-gui` | ninguno | `false` | Reabrir la ventana tras actualizar (Windows) |
| `--wait-pid` | ninguno | `0` | Esperar a que termine este proceso antes de actualizar (Windows) |

### `obit watch`

Para aprobar cada archivo, establece `file_expiry = "propose"` en `[watch]`. Revisa las propuestas con `obit proposals` y aprueba con `obit approve <ID>`. Las subcarpetas caducadas siguen generando propuestas y se quedan en su sitio incluso tras la aprobación en macOS y Linux.

Dar a cada elemento de primer nivel de una carpeta su propia vida útil

Uso: `obit watch <directory> <lifespan> | watch --list | watch --remove <directory> [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--age` | ninguno | `"first-seen"` | Base de la antigüedad: first-seen o modified (requiere aprobación) |
| `--clear-exclusion` | ninguno | `0` | Borrar de forma explícita este ID de exclusión (con --policy) |
| `--help` | `-h` | `false` | Ayuda de watch |
| `--json` | ninguno | `false` | Imprimir JSON |
| `--list` | ninguno | `false` | Listar las políticas de carpeta |
| `--no-daemon` | ninguno | `false` | No iniciar el proceso en segundo plano |
| `--policy` | ninguno | `0` | ID de la política cuya exclusión se debe borrar |
| `--remove` | ninguno | `""` | Quitar una política y cancelar las vidas útiles que registró |
| `--remove-id` | ninguno | `0` | Quitar una política de carpeta por ID, incluso si faltan las rutas |

<!-- generated-command-reference:end -->
