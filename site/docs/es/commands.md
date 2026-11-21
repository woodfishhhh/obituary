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

## Integración y tareas en segundo plano

```powershell
obit init                       # Claude Code hooks and Skill, context menu, Start menu and desktop shortcuts; the tray starts at login
obit init -g --agent codex      # Codex
obit init -g --guard-rm         # delete guard
obit init -g --mcp              # add the MCP server
obit init --no-context-menu --no-shortcuts --no-skill   # skip the context menu, shortcuts or Skill
obit init -g --show             # what is installed
obit daemon enable              # tray at login plus the fallback scheduled task
obit daemon status
obit daemon disable
```

## Consultas y comprobaciones

```powershell
obit doctor              # read-only check (--json)
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

Las tablas siguientes se generan con la salida de `obit --help` construida desde el código actual.

### `obit add`
Alias: `add`, `mark`
Uso: `obit add <path>... <duration> \| add <path>... --at <time> [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--at` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--force` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |
| `--no-daemon` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--note` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

Ejemplos:

```text
obit add debug.log 2h
obit add output\shots 1d12h
obit add report.zip --at "2026-12-01 18:00"
```

### `obit approve`
Uso: `obit approve <proposal id>... [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |

### `obit bury`
Uso: `obit bury <path>... [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--force` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |
| `--note` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit cancel`
Alias: `cancel`, `spare`
Uso: `obit cancel <id\|path>... [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |

### `obit completion`
Uso: `obit completion [command]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |

### `obit completion bash`
Uso: `obit completion bash`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--no-descriptions` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit completion fish`
Uso: `obit completion fish [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--no-descriptions` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit completion powershell`
Uso: `obit completion powershell [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--no-descriptions` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit completion zsh`
Uso: `obit completion zsh [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--no-descriptions` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit config`
Uso: `obit config [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--create` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--language` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit config telemetry`
Uso: `obit config telemetry on\|off [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |

### `obit daemon`
Uso: `obit daemon [command]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |

### `obit daemon disable`
Uso: `obit daemon disable [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--dry-run` | ninguno | `false` | Solo muestra lo que haría, sin cambiar archivos ni servicios. |
| `--help` | `-h` | `false` | Muestra la ayuda. |

### `obit daemon enable`
Uso: `obit daemon enable [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--dry-run` | ninguno | `false` | Solo muestra lo que haría, sin cambiar archivos ni servicios. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--resident` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit daemon start`
Uso: `obit daemon start [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |

### `obit daemon status`
Uso: `obit daemon status [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |

### `obit daemon stop`
Uso: `obit daemon stop [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |

### `obit docker`
Uso: `obit docker [command]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |

### `obit docker add`
Uso: `obit docker add <container name\|ID> <lifespan> [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |

### `obit docker list`
Uso: `obit docker list [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |

### `obit doctor`
Uso: `obit doctor [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |
| `--verbose` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit extend`
Uso: `obit extend <id\|path>... <duration> [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |

Ejemplos:

```text
obit extend 3 1d
obit extend debug.log 30m
```

### `obit git-clean`
Uso: `obit git-clean [options] [--] [pathspec]... [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |

### `obit graveyard`
Alias: `graveyard`, `graves`
Uso: `obit graveyard [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--here` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |

### `obit help`
Uso: `obit help [command] [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |

### `obit hook`
Uso: `obit hook [command]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |

### `obit hook antigravity`
Uso: `obit hook antigravity [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook audit`
Uso: `obit hook audit [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--lines` | `-n` | `40` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook claude`
Uso: `obit hook claude [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook cline`
Uso: `obit hook cline [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook codebuddy`
Uso: `obit hook codebuddy [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook codebuddy-ide`
Uso: `obit hook codebuddy-ide [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook codex`
Uso: `obit hook codex [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook copilot`
Uso: `obit hook copilot [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook cursor`
Uso: `obit hook cursor [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook droid`
Uso: `obit hook droid [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook gemini`
Uso: `obit hook gemini [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook hermes`
Uso: `obit hook hermes [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook kilocode`
Uso: `obit hook kilocode [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook kimi`
Uso: `obit hook kimi [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook lingma`
Uso: `obit hook lingma [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook minimax-code`
Uso: `obit hook minimax-code [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook omp`
Uso: `obit hook omp [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook openclaw`
Uso: `obit hook openclaw [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook opencode`
Uso: `obit hook opencode [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook pi`
Uso: `obit hook pi [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook qoder`
Uso: `obit hook qoder [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook qoder-cn`
Uso: `obit hook qoder-cn [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook trae`
Uso: `obit hook trae [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook trae-cn`
Uso: `obit hook trae-cn [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook vibe`
Uso: `obit hook vibe [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit hook zcode`
Uso: `obit hook zcode [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--event` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--notice-only` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit init`
Uso: `obit init [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--agent` | ninguno | `[claude]` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--context-menu` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--dry-run` | ninguno | `false` | Solo muestra lo que haría, sin cambiar archivos ni servicios. |
| `--format` | ninguno | `"text"` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--global` | `-g` | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--guard-rm` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--mcp` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--no-context-menu` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--no-shortcuts` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--no-skill` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--show` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--skill` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--uninstall` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--yes` | ninguno | `false` | Confirma la operación sin volver a preguntar. |

Ejemplos:

```text
obit init -g                 # Claude Code, for every project (~/.claude)
obit init                    # Claude Code, this project only (.claude/)
obit init -g --agent codex   # Codex (~/.codex/hooks.json + AGENTS.md)
obit init -g --uninstall     # remove only what Obituary added
obit init -g --show          # what is installed
obit init --show --format markdown > docs/agents.md
```

### `obit list`
Alias: `list`, `ls`
Uso: `obit list [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |

### `obit log`
Uso: `obit log [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |
| `--lines` | `-n` | `20` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit mcp`
Uso: `obit mcp [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |

### `obit mktemp`
Uso: `obit mktemp <lifespan> [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |
| `--name` | ninguno | `"tmp"` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--session` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

Ejemplos:

```text
obit mktemp 2h --name screenshots
```

### `obit proposals`
Uso: `obit proposals [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |

### `obit purge`
Alias: `purge`, `cremate`
Uso: `obit purge [id\|original path]... [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--all` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--dry-run` | ninguno | `false` | Solo muestra lo que haría, sin cambiar archivos ni servicios. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |
| `--yes` | ninguno | `false` | Confirma la operación sin volver a preguntar. |

### `obit reject`
Uso: `obit reject <proposal id>... [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |

### `obit remove-item`
Uso: `obit remove-item [-Recurse] [-Force] [-Path\|-LiteralPath] <path> [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |

### `obit resurrect`
Alias: `resurrect`, `raise`
Uso: `obit resurrect <id\|original path>... \| --last [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |
| `--last` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit rm`
Uso: `obit rm [options] <path>... [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |

### `obit rmdir`
Uso: `obit rmdir [options] <path>... [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |

### `obit rules`
Uso: `obit rules [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |

### `obit rules apply`
Uso: `obit rules apply <path>... [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |
| `--session` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit rules check`
Uso: `obit rules check <path>... [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |
| `--session` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit stats`
Alias: `stats`, `eulogy`
Uso: `obit stats [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |

### `obit sweep`
Alias: `sweep`, `reap`
Uso: `obit sweep [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--dry-run` | ninguno | `false` | Solo muestra lo que haría, sin cambiar archivos ni servicios. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |

### `obit trust`
Uso: `obit trust [rules file] [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--yes` | ninguno | `false` | Confirma la operación sin volver a preguntar. |

### `obit uninstall`
Uso: `obit uninstall [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--dry-run` | ninguno | `false` | Solo muestra lo que haría, sin cambiar archivos ni servicios. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--purge-data` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--yes` | ninguno | `false` | Confirma la operación sin volver a preguntar. |

### `obit untrust`
Uso: `obit untrust [rules file] [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Muestra la ayuda. |

### `obit update`
Uso: `obit update [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--apply` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--check` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--relaunch-gui` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--wait-pid` | ninguno | `0` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

### `obit watch`
Uso: `obit watch <directory> <lifespan> \| watch --list \| watch --remove <directory> [flags]`

| Parámetro | Corto | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| `--age` | ninguno | `"first-seen"` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--clear-exclusion` | ninguno | `0` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--help` | `-h` | `false` | Muestra la ayuda. |
| `--json` | ninguno | `false` | Muestra la salida en JSON. |
| `--list` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--no-daemon` | ninguno | `false` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--policy` | ninguno | `0` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--remove` | ninguno | `""` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |
| `--remove-id` | ninguno | `0` | El comportamiento actual de este parámetro se genera desde el código; consulta también la ayuda. |

<!-- generated-command-reference:end -->
