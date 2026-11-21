# Referencia de configuración

Obituary lee `config.toml` desde su directorio de configuración. Todas las claves son opcionales y las omitidas usan los valores predeterminados siguientes. `obit config` muestra la ruta y los valores efectivos.

## Claves

| Clave | Predeterminado | Valores o formato | Efecto |
| --- | --- | --- | --- |
| `codex_windows_shell` | sin definir | `bash`, `powershell` | Declara el shell de Codex en Windows; las eliminaciones ambiguas pasan sin cambios si falta. |
| `agent_shells.<agent>` | sin definir | `bash`, `powershell` | Declara el shell de Windows para un hook agent. |
| `graveyard.mode` | `graveyard` | `graveyard`, `recycle` | Mueve los elementos vencidos al cementerio o a la papelera del sistema. |
| `graveyard.retention_days` | `7` | Entero entre `1` y `3650` | Días durante los que puede restaurarse un elemento. |
| `watch.stable_window` | `10m` | Duración Go, como `30s`, `10m`, `1h` | Espera a que un elemento de una política de carpeta esté estable. Debe ser positiva. |
| `explorer.enabled` | `true` | `true`, `false` | Escribe etiquetas de fecha en el Explorador. |
| `explorer.everywhere` | `false` | `true`, `false` | Muestra fechas en todas las carpetas compatibles. |
| `explorer.in_git` | `false` | `true`, `false` | Muestra fechas dentro de árboles de trabajo Git. |
| `ui.language` | `auto` | `auto`, `en`, `zh-CN` | Idioma del CLI, la ventana y la bandeja. |
| `ui.tray` | `true` | `true`, `false` | Activa la entrada de bandeja o barra de menús. |
| `ui.taskbar` | `true` | `true`, `false` | Da a la ventana de Windows su propio botón. |
| `hooks.session_summary` | `false` | `true`, `false` | Muestra un resumen al iniciar una sesión. |
| `hooks.session_cleanup` | `true` | `true`, `false` | Ordena archivos creados por esta sesión que sigan sin clasificar. |
| `proposals.mode` | `off` | `off`, `tiered`, `auto`, `ask` | Define qué registros automáticos esperan aprobación. |
| `update.check` | `true` | `true`, `false` | Comprueba automáticamente versiones estables. |
| `telemetry.enabled` | `true` | `true`, `false` | Permite enviar estadísticas de uso; es independiente de las actualizaciones. |

`agent_shells` es una tabla por nombre de agent, por ejemplo `[agent_shells] codex = "powershell"`. `codex_windows_shell` es la forma antigua de Codex; los valores distintos producen un error.

## Formatos y errores

Las duraciones usan la sintaxis Go y pueden combinar `ns`, `us`, `ms`, `s`, `m` y `h`, como `1h30m`. La retención debe estar entre `1` y `3650`. `graveyard.mode`, `ui.language` y `proposals.mode` solo aceptan los valores indicados.

Las claves desconocidas no se ignoran. El error incluye ruta, línea y clave completa, y sugiere una clave cercana cuando corresponde. Un TOML inválido también hace fallar `obit config`.

## Operaciones habituales

```powershell
obit config
obit config --create
obit config --language en
obit config telemetry off
obit config telemetry on
```

`--create` escribe la plantilla comentada solo si no existe el archivo. `--language` acepta `auto`, `en` y `zh-CN`. `telemetry on|off` cambia únicamente `telemetry.enabled`; no modifica `[update] check`.
