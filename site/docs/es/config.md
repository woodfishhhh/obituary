# Referencia de configuración

Obituary guarda sus ajustes en `config.toml`, dentro del directorio de configuración. El archivo es opcional: sin él, o con solo unas pocas líneas, todo lo demás usa su valor predeterminado. `obit config` te dice dónde está el archivo, si existe y qué valor tiene realmente cada ajuste en este momento. `obit config --create` escribe un archivo comentado con todos los valores predeterminados, que es la forma más cómoda de empezar a editar.

El directorio de configuración es `%APPDATA%\obituary` en Windows, `~/Library/Application Support/obituary` en macOS y `~/.config/obituary` en Linux (o `obituary` dentro de `XDG_CONFIG_HOME`, si la tienes definida). Para ponerlo en otro sitio, define la variable de entorno `OBIT_CONFIG_DIR`.

| Clave | Valor predeterminado | Valores admitidos |
| --- | --- | --- |
| `graveyard.mode` | `graveyard` | `graveyard`, `recycle` |
| `graveyard.retention_days` | `7` | un entero de 1 a 3650 |
| `graveyard.max_size` | `""` (sin límite) | un tamaño como `500MB`, `5GB`, `10GiB` o bytes; `""`, `0` o `unlimited` desactiva el límite |
| `proposals.mode` | `off` | `off`, `tiered`, `auto`, `ask` |
| `hooks.session_cleanup` | `true` | `true`, `false` |
| `hooks.session_summary` | `false` | `true`, `false` |
| `watch.stable_window` | `10m` | una duración positiva, como `30s`, `10m`, `1h30m` |
| `watch.file_expiry` (macOS/Linux) | `quiet` | `quiet`, `propose` |
| `explorer.enabled` | `true` | `true`, `false` |
| `explorer.everywhere` | `false` | `true`, `false` |
| `explorer.in_git` | `false` | `true`, `false` |
| `ui.language` | `auto` | `auto`, `en`, `zh-CN` |
| `ui.tray` | `true` | `true`, `false` |
| `ui.taskbar` | `true` | `true`, `false` |
| `update.check` | `true` | `true`, `false` |
| `telemetry.enabled` | `true` | `true`, `false` |
| `agent_shells` | sin definir | una tabla: nombre del agente = `bash` o `powershell` |
| `codex_windows_shell` | sin definir | `bash`, `powershell` |

## Adónde va lo que caduca

De forma predeterminada, lo que agota su vida útil va al cementerio de Obituary. Durante `graveyard.retention_days` días (7 si no lo cambias) puedes devolverlo a su sitio con `obit resurrect`; solo pasado ese plazo se elimina para siempre. El número puede ser cualquiera entre 1 y 3650.

En Windows y Linux, `graveyard.mode = "recycle"` usa la papelera del sistema. Restaura los archivos desde la papelera, no con `obit resurrect`. macOS solo admite el cementerio.

`graveyard.max_size` está vacío por defecto (sin límite de capacidad). Si se define un tamaño como `500MB`, `5GB`, `10GiB` o una cantidad de bytes, Obituary cuenta solo el espacio en disco de los archivos y carpetas locales enterrados (sin seguir enlaces simbólicos y excluyendo contenedores). Cuando un entierro normal hace que el total supere el límite, las tumbas más antiguas se purgan primero hasta volver al límite. Si un único elemento supera el límite por sí solo, permanece intacto en su sitio sin vaciar el cementerio y pasa a la cola de decisiones para que elijas eliminarlo permanentemente, ampliarlo, dejar de seguirlo o subir el límite. Reducir el límite con `obit config --max-size` muestra una vista previa y pide confirmación (`--dry-run` solo previsualiza y `--yes` confirma sin preguntar).

## Registro automático y sesiones de agentes

`proposals.mode` decide si Obituary pone vida útil a los archivos por su cuenta. El valor predeterminado, `off`, no hace nada automáticamente: solo entra lo que un agente o tú registráis con comandos, con el menú contextual o con políticas de carpetas. Con `tiered`, los archivos que el agente creó en esta sesión y los contenedores que la sesión puso en marcha entran de inmediato y se te avisa después; con `obit spare` puedes deshacerlo, y todo lo demás espera tu aprobación en `obit proposals`. `auto` registra todo lo que encuentra el registro automático y te avisa después. `ask` te pregunta por cada elemento antes. En cualquier modo, los archivos que Git ya sigue, las dependencias, la salida de compilación y las cachés (`node_modules`, `dist`, `__pycache__` y similares) no se registran nunca de forma automática. Las reglas están en [Qué archivos se registran](./auto-register).

`hooks.session_cleanup` viene activado. Hacia el final de cada sesión se le cuenta al agente qué archivos creó que no tienen vida útil y que Git no sigue, y se le pide que dé vida útil a los temporales (`obit mark`) o que los entierre; los entregables se quedan. `hooks.session_summary` viene desactivado; si lo activas, al empezar la sesión el agente también se entera de qué elementos del proyecto caducan en las próximas 24 horas y qué se enterró hace poco. Está desactivado para ahorrar tokens, porque al agente se le avisa de todos modos cuando toca una ruta enterrada.

## Políticas de carpetas y fechas en el Explorador

`watch.stable_window` es la paciencia de una política de carpetas: un archivo nuevo tiene que permanecer sin cambios durante este tiempo antes de moverse, para que no se lleve un archivo que aún se está descargando. Usa el formato de duración de Go, un número seguido de `s`, `m` o `h`, que se puede encadenar como en `1h30m`, y debe ser mayor que cero.

El modo automático debe poder inspeccionar todos los procesos del propietario del archivo. Solo puede ignorar procesos inaccesibles de otros usuarios, incluido root, si el archivo es tuyo y ni el grupo ni los demás tienen permiso de escritura. Elige `propose` para revisar cada archivo. La aprobación admite información incompleta, pero si se detecta un archivo abierto, se conserva y se muestra el motivo.

Los ajustes `explorer.*` controlan las fechas junto al nombre en el Explorador de Windows y las etiquetas de fecha en Finder y Dolphin. El nombre real no cambia. De forma predeterminada ocurre en Descargas, en el Escritorio (con sus subcarpetas) y en los lugares que gestiona una política de carpetas. Con `explorer.everywhere` activado, también lo muestran otras ubicaciones seguidas; las áreas de trabajo de Git quedan fuera salvo que pongas `explorer.in_git` en `true`; y `explorer.enabled = false` lo desactiva por completo. Consulta [Políticas de carpetas](./folder-policies).

## Interfaz

`ui.language` fija el idioma de la línea de comandos, la ventana, la bandeja y el menú contextual. Por ahora solo existen `en` y `zh-CN`. `auto` mira la variable de entorno `OBIT_LANG`, luego `LC_ALL`, `LC_MESSAGES` y `LANG`, y por último el idioma de la interfaz del sistema. `ui.tray` muestra u oculta el icono de la bandeja (el de la barra de menús en macOS); si está oculto, la limpieza en segundo plano y las comprobaciones de actualización continúan. `ui.taskbar` solo se aplica a Windows y decide si la ventana ocupa un botón en la barra de tareas mientras está abierta.

## Comprobación de actualizaciones y estadísticas de uso

Con `update.check` activado, Obituary lee la información pública de versiones en GitHub como máximo una vez al día y no envía datos de uso. Si lo desactivas, siempre puedes ejecutar `obit update --check` cuando quieras. `telemetry.enabled` controla las estadísticas de uso anónimas. Es un interruptor distinto del de las actualizaciones: `obit config telemetry off` solo desactiva las estadísticas y borra de este equipo el ID de instalación y cualquier informe pendiente. Los detalles están en la [política de privacidad](./privacy).

## Qué shell usa un agente en Windows

La guardia de eliminación tiene que leer el comando de borrado que un agente está a punto de ejecutar para poder ofrecer el comando `obit` equivalente. En Windows un agente puede usar bash o PowerShell, y Obituary no sabe cuál, así que lo declaras tú: este agente se ejecuta en este equipo Windows, con este shell. Sin declaración, los comandos de significado ambiguo pasan sin bloquearse.

```toml
[agent_shells]
codex = "powershell"
```

Las claves de `agent_shells` son nombres de agentes. Ahora mismo la leen `codex`, `codebuddy`, `codebuddy-ide` y `minimax-code`. `codex_windows_shell` es la forma antigua, solo para Codex; si ella y `agent_shells.codex` están definidas con valores distintos, la configuración se rechaza.

## Cómo se ve un error

Obituary no ignora en silencio una clave que no conoce. Si lo hiciera, `retention = 30` dejaría vigente la retención de 7 días mientras tú crees que ha pasado a 30. Por eso `obit config` informa de la ruta del archivo, el número de línea y el nombre completo de lo que no reconoce, y sugiere la clave correcta cuando la ortografía se parece:

```text
obit: config file has errors: …\config.toml:2: unknown key "graveyard.retention" (did you mean "graveyard.retention_days"?)
```

Un valor fuera de rango se notifica igual, con el ajuste y lo que admite, por ejemplo `graveyard.retention_days must be between 1 and 3650`. Un error de sintaxis TOML también hace que `obit config` falle. Corrige el archivo y vuelve a ejecutarlo; cuando deje de quejarse, está arreglado.

## Comandos habituales

```powershell
obit config                  # ubicación del archivo, directorio de datos y valores en vigor
obit config --create         # escribe el archivo predeterminado con comentarios
obit config --language en    # guarda el idioma de la interfaz: auto, en o zh-CN
obit config telemetry off    # desactiva las estadísticas de uso (on las vuelve a activar)
```

`--create` nunca sobrescribe un archivo existente; si el archivo ya está, informa de un error y se detiene. `--language` crea el archivo con los valores predeterminados si todavía no existe.
