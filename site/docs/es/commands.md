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
<!-- generated-command-reference:end -->
