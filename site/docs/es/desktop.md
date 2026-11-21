# Ventana, bandeja del sistema y menú contextual

Puedes resucitar archivos, aprobar propuestas y cambiar ajustes sin usar la línea de comandos.

## Ventana pequeña e icono de la barra de menús en macOS

Descarga el DMG universal de macOS y el archivo de checksums de la misma versión. Comprueba SHA-256, abre el DMG y arrastra Obituary.app a Aplicaciones o a la carpeta Applications del usuario; expulsa el DMG antes de activar el inicio de sesión.

La app contiene la ventana Wails, el icono de la barra de menús y los auxiliares obit y obitd. El agente de usuario launchd que comprueba cada 5 minutos sigue siendo un respaldo independiente. En Ajustes, activa el inicio del tray al iniciar sesión para instalar el LaunchAgent de Obituary; al desactivarlo solo se elimina esa entrada.

Al eliminar `Obituary.app`, tus datos permanecen por defecto en `~/Library/Application Support/obituary/`: `store.json` contiene el registro, `config.toml` los ajustes y `graveyard/` los archivos enterrados en ese volumen.

### Primer inicio de una app sin firma

La versión no tiene firma Developer ID ni notarización. Confirma el origen y SHA-256, e intenta abrirla una vez. Si macOS no puede verificar al desarrollador, abre «Ajustes del Sistema → Privacidad y seguridad», elige «Abrir igualmente» para Obituary, confirma y vuelve a abrirla. No desactives Gatekeeper globalmente ni borres la cuarentena de forma recursiva. Detente ante avisos de app dañada, malware o políticas de la organización.

### Propuesta para Homebrew Cask

El cask debe instalar el mismo DMG universal, copiar Obituary.app a Aplicaciones y exponer obit y obitd desde el app bundle. Hasta que el DMG de escritorio tenga una URL estable y una comprobación de desinstalación revisada, se mantiene separado del cask existente para CLI.
## Bandeja del sistema

`obit init` hace que aparezca un icono en la bandeja al iniciar sesión. Desde su menú puedes:

- ver los entierros recientes y pulsar en uno para resucitarlo;
- consultar los elementos que caducan en las próximas 24 horas;
- pausar o reanudar la limpieza automática;
- abrir la ventana con «Open Obituary» (si `obit-gui.exe` no está junto a `obitd.exe`, una notificación explica el problema);
- salir de la bandeja sin detener la tarea programada.

## La pequeña ventana

`obit-gui.exe` tiene cinco páginas: Upcoming, Graveyard, Decisions, Settings y Logs. Puedes abrir la ventana desde «Obituary» en Inicio, desde el acceso directo del escritorio o con «Open Obituary» en la bandeja.

- La barra superior de la página de inicio indica «Auto registration: off» u «on». Púlsala para abrir los ajustes. Tanto en la parte superior de Decisions como en Settings puedes elegir entre desactivado (por defecto: solo entra lo que registráis tú y el agente), escalonado, totalmente automático o preguntar siempre. Consulta [Qué se registra](./auto-register). Más abajo, «Handled automatically» muestra lo registrado sin preguntar. Con un clic puedes deshacer el registro (se detiene la cuenta atrás y el elemento queda en su sitio) o prolongar la vida útil.
- Si un elemento caduca pero no se puede enterrar, por ejemplo porque otro programa mantiene abierto el archivo, aparece en «Burials that failed» de Decisions, con el motivo y la fecha del siguiente intento. Obituary reintenta a intervalos cada vez mayores. Tras varios fallos se detiene: «Retry» vuelve a intentarlo ahora y «Stop tracking» deja el elemento en su sitio sin seguirlo.
- Mientras Docker no esté en ejecución, no podrás aprobar propuestas de contenedores. La tarjeta explica el motivo. Podrás aprobarlas cuando Docker esté en marcha.
- Al confiar en un archivo `.obitrules`, solo confías en la versión exacta que aparece en la ventana.
- La ventana, la bandeja y el menú contextual usan `[ui] language` de `config.toml` (`auto`, `zh-CN` o `en`; `auto` sigue el idioma de Windows). También puedes cambiarlo en los ajustes.
- La ventana solo está disponible para x64 y requiere el entorno de ejecución de WebView2, incluido en Windows 11 y presente en la mayoría de los equipos con Windows 10.

### Actualizaciones

Instala una versión estable más reciente desde Settings → About and updates. Obituary cierra la ventana y la bandeja, actualiza por el canal de instalación original (Scoop, WinGet o script) y luego intenta restaurar la bandeja y volver a abrir la ventana. Si falla, la misma página muestra el error y la ruta del log.

Una copia que descomprimiste tú mismo también se actualiza en su sitio; su botón dice «Download and restart to update». Obituary descarga la versión desde GitHub (desde el espejo de China continental si GitHub no responde en diez segundos), comprueba su SHA-256 y solo entonces cierra la ventana y reemplaza los programas propios de Obituary en esa carpeta (`obit`, `obitd` y `obit-gui`). Los demás archivos de la carpeta no se tocan, y tampoco PATH ni el menú Inicio. Los programas anteriores esperan un día en una carpeta oculta `.obituary-update-…` del mismo lugar y luego pasan al cementerio, donde aún se pueden resucitar; si falla cualquier paso, incluido un `obit` nuevo que no arranca, se restaura la versión anterior. Hace falta una versión estable (no una compilación de desarrollo), todos los programas de la versión en la carpeta como archivos normales y una carpeta donde el usuario actual pueda escribir sin permisos de administrador. Si algo de esto no se cumple, la página indica qué y puedes descargar la copia nueva desde la página de versiones. Desde un terminal, `obit update --apply` hace lo mismo. «What changed» muestra las notas en el idioma de la ventana, o en inglés si no hay traducción.

### Ocho estilos

Elige uno en Settings → Appearance. El cambio es inmediato y solo afecta a la ventana de este equipo.

| Estilo | Descripción |
|---|---|
| Calendario | Una columna por día. Cada archivo aparece en la fecha de su entierro o eliminación definitiva. Arrástralo a una fecha posterior para ampliar su vida útil. |
| Una línea | Un solo campo de entrada, todo desde el teclado: busca un archivo y pulsa Intro para resucitarlo o ampliar su vida útil. |
| Túnel | Un túnel temporal en 3D. Cuanto más cerca está un archivo, antes caduca. Usa la rueda para recorrer el tiempo. |
| Vigilia | Fondo oscuro con una luz por archivo. Cuanto más tenue es la luz, menos tiempo queda. |
| Esquelas | La página de esquelas de un periódico, con columnas en letra con serifa y un sello al resucitar un archivo. |
| Horizonte | Los archivos en seguimiento aparecen sobre el suelo; el cementerio, debajo. |
| Mica | Apariencia nativa de Windows 11 con una barra lateral. |
| Noche | Tema oscuro y discreto con una barra superior. |

## Menú contextual

`obit init` instala por defecto el menú contextual del Explorador solo para el usuario actual. Haz clic con el botón derecho en un archivo o carpeta, elige «Add to Obituary» y selecciona:

- **1 día / 7 días / 30 días**: se entierra al caducar; hasta entonces puedes usarlo normalmente.
- **Bury now (recoverable)**: va al cementerio de inmediato. Puedes resucitarlo durante la retención, que por defecto es de siete días.

Puedes seleccionar varios archivos a la vez. Una notificación muestra cuántos se añadieron, cuándo se enterrarán y cuántos fallaron. En el menú nuevo de Windows 11 elige «Mostrar más opciones», o mantén Mayús al hacer clic con el botón derecho. `obit init --dry-run` enumera las entradas con sus comandos y `obit doctor` comprueba la instalación.

macOS y Linux todavía no tienen menú contextual. Usa `obit mark <path> 7d` para registrar un elemento y `obit bury <path>` para enterrarlo de inmediato.

## Iconos y menús

`obit init` configura estas opciones de forma predeterminada. Cada una tiene su propio interruptor en Settings → Icons and menus:

| Opción | Qué controla |
|---|---|
| Icono de la bandeja | Muestra Obituary en el área de notificación. Ocultarlo no detiene la limpieza. |
| Icono de la barra de tareas | Si la ventana ocupa un lugar en la barra de tareas mientras está abierta. |
| Menú Inicio | La entrada «Obituary» en Inicio. |
| Acceso directo del escritorio | «Obituary» en el escritorio. |
| Menú contextual | La entrada «Add to Obituary» del Explorador. |

Para omitirlos durante la instalación, usa `obit init --no-context-menu` para el menú contextual y `--no-shortcuts` para los accesos directos de Inicio y del escritorio.

Las fechas de entierro del Explorador se explican en [Políticas de carpetas](./folder-policies#dates-in-explorer).
