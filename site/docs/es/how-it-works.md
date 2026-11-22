# Cómo funciona Obituary

## La vida de un archivo

1. **Registrar**: asigna una vida útil al archivo o carpeta. El agente usa `obit add debug.log 2h`; tú puedes elegir «Add to Obituary» en el menú contextual del Explorador. Consulta [Qué se registra](./auto-register).
2. **Caducar**: `obit list` muestra cuánto tiempo queda. Antes de la fecha límite, `obit extend` amplía el plazo y `obit spare` deja de hacer seguimiento sin eliminar el archivo.
3. **Enterrar**: cuando acaba el plazo, el elemento pasa al cementerio (`obit graveyard`), donde se conserva siete días por defecto.
4. **Resucitar o eliminar definitivamente**: durante la retención, `obit resurrect <id>` devuelve el elemento a su sitio sin sobrescribir nada. Solo al terminar el plazo se elimina definitivamente (`obit purge`).

¿No sabes si aún necesitas algo? Entiérralo ahora con `obit bury`. Durante siete días podrás recuperarlo tal como estaba.

## Dónde está el cementerio

El cementerio está en la misma unidad que el archivo. En la unidad del directorio de datos se usa el cementerio de ese directorio; en otras unidades se usa la carpeta oculta `.obituary` de la raíz. Por eso enterrar solo requiere cambiar el nombre: es rápido y no puede quedar a medio hacer.

El plazo de retención se ajusta en la configuración. También puedes enviar los elementos directamente a la papelera del sistema. En ese caso debes restaurarlos desde la papelera, no con `obit resurrect`:

```toml
[graveyard]
mode = "graveyard"      # or "recycle": straight to the system Recycle Bin
retention_days = 7      # days the graveyard keeps things
```

El archivo de configuración es `%APPDATA%\obituary\config.toml`. `obit config` muestra la configuración y `obit config --create` escribe los valores predeterminados con comentarios.

## Cómo se evita eliminar otros archivos

- **Solo la purga elimina definitivamente.** Cada entierro crea una carpeta de tumba propia y registra el ID de esa carpeta. Antes de eliminar o resucitar, Obituary comprueba que la carpeta siga siendo la misma, que tenga un único elemento y que ese elemento sea el objeto enterrado. Ni un registro dañado ni un cementerio sustituido por un enlace pueden provocar que se borre algo fuera del cementerio.
- **Solo se resucita en la ubicación original.** Si el nombre está ocupado, no se sobrescribe nada. El elemento se restaura al lado con un nombre como `name (resurrected).ext`; se conservan ambos y el resultado muestra la ruta real. Una carpeta superior eliminada se vuelve a crear. Si se ha convertido en una unión de directorios o un enlace simbólico, si la unión apunta a otro destino o si la letra de unidad corresponde a otro volumen, se rechaza la operación y la tumba queda intacta.
- **Se identifica el archivo, no el nombre.** Cada entrada guarda el número de serie del volumen y el ID de archivo de 128 bits. Así se localiza el archivo aunque cambie de nombre o ubicación, sin tocar otro archivo que ocupe su ruta anterior. La identidad se verifica de nuevo justo antes de eliminar. No se aceptan archivos con más de un enlace duro.
- **Rutas protegidas.** Se rechazan las raíces de unidad, `C:\Windows`, Program Files, ProgramData, el perfil de usuario y carpetas como Escritorio, Documentos y Descargas, así como sus directorios superiores, incluso si están redirigidos. Sí puedes asignar vida útil a sus contenidos.
- **Repositorios Git.** Si una carpeta contiene `.git` a cualquier profundidad, `obit add` y `obit bury` requieren `--force`. La comprobación se repite al caducar. Si no se puede completar, se rechaza el entierro.
- **En caso de duda no se elimina.** Solo se descarta una entrada cuando se sabe que el archivo ya no existe. Si se deniega el acceso o se desconecta un volumen, se vuelve a intentar más tarde. El intervalo empieza en un minuto y aumenta hasta un día. Tras 12 fallos consecutivos, Obituary se detiene y espera tu decisión. Verás el motivo en `obit list` y en «Burials that failed» de la página For you. `obit extend <id> 1m` («Retry») reintenta; `obit spare <id>` («Stop tracking») deja el elemento en su sitio. Antes de enterrar una carpeta grande, Obituary busca repositorios Git durante un máximo de 30 segundos. Solo la entierra si termina la búsqueda sin encontrar ninguno. Si no termina, reintenta más tarde; si hay un repositorio, no la entierra.
- **Queda constancia de cada operación.** Las altas, las ampliaciones, las cancelaciones y las eliminaciones se anotan en el registro de auditoría (`obit log`).

## Ejecución en segundo plano

Con la supervisión de escritorio activada, el proceso de la bandeja atiende los vencimientos y una tarea independiente comprueba los pendientes cada hora. Las ediciones sin bandeja mantienen el intervalo de cinco minutos.

Windows usa una tarea al iniciar sesión, macOS un LaunchAgent de usuario y Linux un servicio systemd de usuario en la sesión gráfica. Tras un cierre inesperado, espera 60 segundos antes de reiniciar; al tercer fallo en 15 minutos deja de intentarlo.

Los hooks del agente también hacen una pasada al iniciar y terminar la sesión; con `obit reap` puedes hacerla ahora.

```powershell
obit daemon enable              # tray at login plus the fallback scheduled task (--dry-run lists what would be installed)
obit daemon status              # current mode and the next expiry (--json)
obit daemon disable             # remove the scheduled task and login start, and stop the running daemon (--dry-run previews)
```

Las consultas `list`, `graveyard`, `stats` y `log` son de solo lectura. No entierran nada.

## Dónde se guardan los datos {#where-the-data-lives}

En `%LOCALAPPDATA%\obituary` se encuentran `store.json` (registro), `audit.jsonl` (log de auditoría), `obitd.log` (log del demonio) y `graveyard\` (cementerio de esta unidad).

Puedes cambiar las ubicaciones con `OBIT_HOME` (datos), `OBIT_CONFIG_DIR` (configuración) y `OBIT_USER_HOME` (directorio personal).
