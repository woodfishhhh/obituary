# Guardia de eliminación

Borrar el directorio personal o una unidad completa ya no es una hipótesis. Con la guardia activa, si el agente ejecuta `rm -rf build`, `rmdir`, `git clean -fdx`, `Remove-Item -Recurse` o una eliminación parecida, Obituary detiene la ejecución y le indica el comando equivalente, como `obit rm -rf build`. Al usarlo, los elementos van al cementerio en vez de desaparecer y `obit resurrect <id>` los devuelve.

```powershell
obit init -g --guard-rm                 # Claude Code; add --agent codex for Codex. Off by default
```

La protección contra borrados reconoce `docker volume rm`, `docker volume remove`, `docker volume prune`, `docker compose down -v` (o `--volumes`) y `docker system prune --volumes`. Solo sugiere `obit bury --docker-volume` para un `docker volume rm` sencillo con nombres explícitos.

La protección contra borrados también detiene `wsl --unregister` (incluidos `wsl.exe`, rutas completas, la forma con `=` y las invocaciones mediante `cmd /c` o `powershell -Command`), que destruye de forma permanente toda una distribución de WSL y su disco virtual (`ext4.vhdx`). Obituary no entierra ni resucita distribuciones de WSL; el rechazo indica `wsl --export <distro> <backup.tar>` como copia de seguridad previa a la confirmación manual. Los comandos de solo lectura o no destructivos como `wsl --list`, `wsl --status`, `wsl --export`, `wsl --terminate` y `wsl --shutdown` pasan sin cambios.

## Una ejecución real

Con la guardia activa, el hook recibió una llamada Bash de Claude Code, `rm -rf build`, y respondió así; `rm` nunca se ejecutó:

```json
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Obituary's delete guard is on: `'C:/work/obit.exe' rm -rf build` provides restorable deletion; `obit resurrect` restores graveyard entries."}}
```

El agente cambia entonces a Obituary: la carpeta va al cementerio y un comando la devuelve.

```console
$ obit rm -rf build
$ obit graveyard --here
ID  BURIED            PURGE  SIZE  PATH
#2  2026-11-11 00:36  in 7d  23 B  C:\work\build
$ obit resurrect --last
#2 resurrected  C:\work\build
```

## Principios

- **Rechazar, no reescribir.** Obituary no cambia el comando del agente. Claude Code comprueba los permisos contra el comando reescrito, así que podría dejar de aplicarse una regla `deny` para el original. Al rechazar el original y dejar que el agente emita el equivalente, todas las reglas de permisos siguen vigentes.
- **Se detiene toda eliminación literal.** Si `rm`, `rmdir`, `git clean` o `Remove-Item` (incluidos `del`, `rd` y `ri`) aparece en cualquier posición de comando, después de `&&`, `||` o `;`, en una tubería, `$(…)`, subshell, condición o bucle, se detiene la ejecución. Por ejemplo: `rm -rf build && npm run build`, `rm -rf build 2>/dev/null` y `git clean -fdx && npm ci`. No cuenta `rm` dentro de cadenas, comentarios o argumentos de `echo`, ni las previsualizaciones `rm --help`, `git clean -n` y `Remove-Item -WhatIf`.
- **Enumera primero los objetivos seleccionados en tiempo de ejecución.** Se rechazan sin reescritura `find -delete`, `find -exec rm` (incluido `-execdir`), `xargs rm` y `Get-ChildItem … | Remove-Item` de PowerShell. Revisa primero las coincidencias y pasa después las rutas explícitas a `obit rm -- <path>...`. Un `find` que solo busca o enumera rutas sí se ejecuta. La guardia no inspecciona envoltorios arbitrarios como `sudo rm` o `bash -c '…'`.
- **Nunca se hace una eliminación real.** Cada ruta entregada a `obit rm`, `obit remove-item` u `obit git-clean` se entierra. Si se rechaza una ruta, el error usa el formato de `rm` y el código de salida es distinto de cero.
- **Se comprueba que el comando no haya cambiado.** Entre la intención del agente y la ejecución intervienen comillas, escapes y a veces un proxy. Si se pierde un carácter, `rm -rf "$OUT"/` puede convertirse en `rm -rf /`. Antes de actuar, `obit rm` verifica la ruta real del disco.
- **En Windows Codex también se inspeccionan las sustituciones entre comillas.** La orden pasa por otro shell y puede ejecutarse texto que Bash habría tratado literalmente. Las expresiones `$(…)` se interpretan según Bash y PowerShell incluso en comillas, heredocs citados y comentarios. Cualquier eliminación detiene la ejecución; el texto con backticks puede escribirse en un archivo y pasarse como ruta.
- Los archivos rastreados por Git también se entierran para que los cambios sin commit puedan recuperarse; es una eliminación deliberada del agente, no un registro automático.
- `obit rm -rf` sobre una carpeta con `.git` entierra todo el repositorio. Un `rm -rf` real lo destruiría, mientras que el entierro puede deshacerse. Las protecciones de carpetas del sistema y del usuario siguen vigentes.
- **El comando equivalente es deliberadamente limitado.** Nunca puede seleccionar más que el original; por eso Obituary no traduce entre shells y solo ofrece equivalentes para las formas de la tabla. En los demás casos detiene el borrado y explica que `obit bury <path>` y `obit git-clean` permiten una eliminación recuperable.

## Lo que se detiene y lo que pasa

| Host | Se detiene con un comando equivalente | Se detiene con el motivo y orientación general | Pasa |
|---|---|---|---|
| Claude Code (herramienta Bash) | `rm`, `rmdir`, `git clean`, opcionalmente precedidos por un `cd x` literal unido con `&&`, `;` o salto de línea; `rm -f *.log` si ningún archivo comienza por `-`, `rm -rf ../dist`, `rm -- "$target"` con la variable después de `--` | Cualquier otra eliminación literal: otras instrucciones (`npm …`, `trap`, `hash`), variables antes de `--`, rutas POSIX absolutas que comienzan por `/`, redirecciones, `\|\|`, tuberías, `$(…)`, eliminaciones interactivas o `/usr/bin/rm` con ruta | Comandos que no eliminan |
| Codex, bajo `/mnt/<drive>` de WSL | Un único `rm` o `rmdir` cuyos argumentos sean rutas relativas sin `..`, `~`, comodines ni comillas | Cualquier otra eliminación literal, incluido `git clean` y rutas del sistema de archivos propio de WSL | Un directorio de trabajo dentro del sistema de archivos propio de WSL |
| Codex declarado como Windows PowerShell | Un único `Remove-Item` o alias con una ruta literal (se permiten `-Recurse` y `-Force`) | Cualquier otra eliminación literal: varias instrucciones, tuberías, parámetros como `-ErrorAction`, rutas de proveedor, `~`, comodines o backticks | Texto dentro de comillas curvas |
| Codex en Windows con shell desconocido | — | Eliminación dentro de fragmentos raw de backtick o `$(…)` | Otros comandos |
| pwsh en Linux, `git rm` | — | — | Todo pasa |

Para ofrecer un equivalente, la eliminación debe ser el **último segmento** del comando y los segmentos anteriores solo pueden unirse con `&&`, `;` o saltos de línea. No se ofrece para nombres que terminan en punto o espacio, porque Windows los normaliza. La eliminación literal sigue bloqueándose.

## Úselo usted mismo

`obit rm`, `obit rmdir` y `obit git-clean` aceptan las opciones no interactivas habituales de `rm`, `rmdir` y `git clean`. Las opciones no admitidas, como `-i` o `rmdir -p`, producen un error. `obit remove-item` es una variante simple de `Remove-Item` que acepta una sola ruta literal; para comodines usa `obit rm *.log` en Bash.

Desactívala temporalmente con `OBIT_GUARD_RM=0`. `obit stats` muestra cuántas eliminaciones se detuvieron y cuánto se enterró.
