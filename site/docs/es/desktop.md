# Ventana, bandeja del sistema y menú contextual

Puedes resucitar archivos, aprobar propuestas y cambiar ajustes sin usar la línea de comandos.

## Qué tiene cada una de las tres plataformas

La ventana, la bandeja y las notificaciones de escritorio existen en Windows, macOS y Ubuntu 24.04; el menú contextual, las etiquetas de fecha del Explorador y el modo Papelera de reciclaje existen solo en Windows. Las ediciones de escritorio para macOS y Ubuntu son, por ahora, versiones preliminares. Otras distribuciones de Linux no tienen edición de escritorio y usan la línea de comandos.

| | Windows | macOS | Ubuntu 24.04 |
| --- | --- | --- | --- |
| Ventana | `obit-gui.exe`, solo x64 | `Obituary.app`, universal para Intel y Apple Silicon | un paquete `.deb`, solo amd64 |
| Bandeja | área de notificación | barra de menús | bandeja; requiere un escritorio que ofrezca StatusNotifierItem |
| Bandeja al iniciar sesión | activada de forma predeterminada tras `obit init` | interruptor en Settings | interruptor en Settings |
| Comprobación de caducidad | proceso residente de la bandeja, con el Programador de tareas como respaldo | agente de usuario de launchd, cada 5 minutos | temporizador de usuario de systemd, cada 5 minutos |
| Notificaciones de escritorio | sí | sí | sí |
| Menú contextual | en el Explorador | no | no |
| Etiquetas de fecha en el Explorador | sí | no | no |
| Modo Papelera de reciclaje | sí | no, solo cementerio | no, solo cementerio |
| Se instala con | el script de instalación, Scoop, winget o el archivo comprimido | el DMG | el `.deb` |
| Primera apertura | un archivo descargado a mano puede mostrar un aviso de SmartScreen | requiere aprobación manual, ver [más abajo](#primer-inicio-de-una-app-sin-firma) | se abre directamente |
| Requisitos | el runtime de WebView2 | la 1.6.0 necesita macOS 26 (corregido en la 1.6.1) | GTK 3 y WebKitGTK 4.1, que instala apt |

## Ventana pequeña e icono de la barra de menús en macOS

Descarga el DMG universal (`obituary_<versión>_darwin_universal.dmg`) y `checksums.txt` desde la página de la versión, comprueba el SHA-256, abre el DMG, arrastra Obituary.app a Aplicaciones, expulsa el DMG y abre la app desde Aplicaciones. Arrástrala primero porque la entrada de inicio al iniciar sesión guarda dónde está la app, y una app ejecutada directamente desde el DMG ya no está allí cuando se expulsa.

La app contiene la ventana, el icono de la barra de menús y los programas `obit` y `obitd` que la acompañan. De la caducidad se ocupa otro agente de usuario de launchd que se ejecuta cada 5 minutos; lo instalas con `obit daemon enable`, y cerrar la ventana o el icono de la barra de menús no le afecta. Al activar «Start the tray at sign-in» en Settings de la ventana se instala el LaunchAgent de usuario propio de Obituary para que el icono de la barra de menús aparezca al iniciar sesión; al desactivarlo, se elimina solo esa entrada.

Al eliminar `Obituary.app`, tus datos permanecen por defecto en `~/Library/Application Support/obituary/`: `store.json` contiene el registro, `config.toml` los ajustes y `graveyard/` los archivos enterrados en ese volumen.

### Primer inicio de una app sin firma

La versión no tiene firma de Developer ID ni está notarizada, así que macOS la detiene la primera vez. Después de comprobar que la descarga viene de una página de versiones de confianza y de verificar el SHA-256, ábrela normalmente una vez. Si el sistema dice que no puede verificar al desarrollador, abre Ajustes del Sistema, luego Privacidad y seguridad, busca Obituary al final de la página, elige «Abrir igualmente», confirma otra vez y abre la app de nuevo. No desactives Gatekeeper de forma global para esto ni borres el atributo de cuarentena de forma recursiva. Si el sistema indica que la app está dañada o contiene malware, o que una política de la empresa impide abrirla, detente ahí.

### La 1.6.0 necesita macOS 26

La ventana de macOS de la 1.6.0 necesita macOS 26. En macOS 15 y anteriores, el sistema dice que la app no es compatible con esta versión y no se abre. Está corregido en la 1.6.1.

## Ubuntu 24.04 (amd64)

Descarga el paquete de escritorio `obituary_<versión>_linux_amd64.deb` y `checksums.txt` desde la página de la versión. Desde el directorio de descarga, ejecuta los comandos siguientes, sustituyendo `VERSION` por la versión que descargaste; el paquete solo se instala si la comprobación tiene éxito:

```bash
file=obituary_VERSION_linux_amd64.deb
grep -F "  $file" checksums.txt | sha256sum --check --strict - && sudo apt install "./$file"
```

Después abre Obituary desde la lista de aplicaciones. La ventana usa GTK 3 y WebKitGTK 4.1, y apt los instala junto con el paquete. Los programas están en `/usr/libexec/obituary/`, que no está en tu PATH; para usar `obit` en una terminal, instala la línea de comandos por separado con el script de instalación.

La bandeja necesita un escritorio que ofrezca StatusNotifierItem, y GNOME no lo hace de forma predeterminada. Allí abres la misma ventana desde la lista de aplicaciones, sin ninguna extensión, pero no hay icono en la bandeja. Para que la bandeja aparezca en tu próximo inicio de sesión, activa «Start the tray at sign-in» en Settings; al desactivarlo se elimina la entrada de inicio que escribió Obituary. Para desinstalar, consulta [Desinstalación](./uninstall).

## Bandeja del sistema

`obit init` hace que aparezca un icono en la bandeja al iniciar sesión. Desde su menú puedes:

- ver los entierros recientes y pulsar en uno para resucitarlo;
- consultar los elementos que caducan en las próximas 24 horas;
- pausar o reanudar la limpieza automática;
- abrir la ventana con «Open Obituary» (si `obit-gui.exe` no está junto a `obitd.exe`, una notificación explica el problema);
- salir de la bandeja sin detener la tarea programada.

## La pequeña ventana

`obit-gui.exe` tiene cinco páginas: Expiring, Graveyard, For you, Settings y Logs. Puedes abrir la ventana desde «Obituary» en Inicio, desde el acceso directo del escritorio o con «Open Obituary» en la bandeja.

- La barra superior de la página de inicio indica «Auto registration: off» u «on». Púlsala para abrir los ajustes. Tanto en la parte superior de For you como en Settings puedes elegir entre desactivado (por defecto: solo entra lo que registráis tú y el agente), escalonado, totalmente automático o preguntar siempre. Consulta [Qué se registra](./auto-register). Más abajo, «Handled automatically» muestra lo registrado sin preguntar. Con un clic puedes deshacer el registro (se detiene la cuenta atrás y el elemento queda en su sitio) o prolongar la vida útil.
- Si un elemento caduca pero no se puede enterrar, por ejemplo porque otro programa mantiene abierto el archivo, aparece en «Burials that failed» de For you, con el motivo y la fecha del siguiente intento. Obituary reintenta a intervalos cada vez mayores. Tras varios fallos se detiene: «Retry» vuelve a intentarlo ahora y «Stop tracking» deja el elemento en su sitio sin seguirlo.
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

Si no los quieres al instalar, añade `obit init --no-context-menu` para omitir el menú contextual o `--no-shortcuts` para omitir los accesos directos del menú Inicio y del escritorio. Las mismas opciones valen al desinstalar: `obit init --uninstall --no-shortcuts` deja los accesos directos existentes y retira lo demás como de costumbre, y `--no-context-menu` hace lo mismo con el menú contextual.

Las fechas de entierro del Explorador se explican en [Políticas de carpetas](./folder-policies#dates-in-explorer).
