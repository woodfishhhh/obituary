# Solución de problemas

Cuando algo falla, empieza por `obit doctor`. Solo lee y nunca escribe. Comprueba la instalación, las integraciones con agentes, la programación en segundo plano y tus datos, y cada línea empieza por `PASS`, `INFO`, `WARN` o `FAIL`, seguida del motivo y de lo que conviene hacer; si hay algún `FAIL`, el comando termina con un código de salida distinto de cero. Añade `--json` para pasarle el resultado a otra persona, o `--verbose` para ver exactamente qué funciones tiene cada agente.

Cada apartado sigue el mismo orden: qué ves, por qué ocurre, qué hacer y cómo saber que está resuelto.

## La terminal no encuentra `obit`

La terminal dice `command not found`, o PowerShell no reconoce `obit`. El programa está en el disco, pero su carpeta no está en tu PATH. El script de instalación de Windows añade `%LOCALAPPDATA%\Programs\obituary` al PATH de tu usuario, pero una terminal que ya estaba abierta no ve el PATH nuevo. `install.sh` deja los programas en `~/.local/bin` y nunca edita el PATH, y en algunos sistemas esa carpeta tampoco está en el PATH de partida. El `.deb` de Ubuntu los pone en `/usr/libexec/obituary/`, que tampoco está en el PATH.

En Windows, cierra la terminal y abre una nueva. En macOS y Linux, añade `~/.local/bin` a la configuración de tu shell; en zsh, escribe `export PATH="$HOME/.local/bin:$PATH"` en `~/.zshrc` y abre una terminal nueva. Si solo instalaste el `.deb`, usa la ruta completa `/usr/libexec/obituary/obit`, o instala la línea de comandos por separado con el script de instalación.

Sabrás que está resuelto cuando `obit --version` muestre una versión y la línea `path` de `obit doctor` deje de ser `WARN`.

## `obit doctor` no encuentra `obitd`

Una línea `FAIL obitd` en `obit doctor` significa que el programa en segundo plano `obitd` no está junto a `obit`. Obituary lo busca en la misma carpeta, así que copiar solo `obit` no basta. Deja `obit` y `obitd` juntos, o vuelve a ejecutar el instalador de la forma en que instalaste. Cuando `obit doctor` muestre `PASS` en esa línea, está resuelto.

## Un archivo descargado no supera la suma de comprobación

Si usas el script de instalación, él mismo hace esta comprobación y se detiene sin instalar nada si el hash no coincide. Solo tienes que comprobarlo a mano cuando descargaste tú mismo un archivo comprimido, un DMG o un `.deb`. Una discrepancia suele significar que la descarga está incompleta, que el archivo se alteró por el camino o que `checksums.txt` es de otra versión, ya que los hashes son de cada versión.

Descarga de nuevo el paquete y `checksums.txt` desde la misma página de la versión (desde el espejo, si estás en China continental) y compruébalo otra vez; si sigue sin coincidir, no lo ejecutes. Sustituye `VERSION` y el nombre del archivo por los de lo que descargaste. En Linux:

```bash
file=obituary_VERSION_linux_amd64.deb
grep -F "  $file" checksums.txt | sha256sum --check --strict -
```

En macOS, cambia `sha256sum --check --strict -` de la última línea por `shasum -a 256 -c -`. En Windows PowerShell:

```powershell
$file = "obituary_VERSION_windows_amd64.zip"
(Get-FileHash $file -Algorithm SHA256).Hash -eq ((Select-String -Path checksums.txt -SimpleMatch $file).Line -split '\s+')[0]
```

Cuando los hashes coinciden, Linux y macOS imprimen el nombre del archivo seguido de `OK`, y PowerShell imprime `True`.

## Windows muestra «Windows protegió tu PC»

Windows SmartScreen puede detener un archivo comprimido que descargaste a mano con el navegador, porque los programas de Obituary todavía no tienen firma de código. Con Scoop, winget o el script de instalación casi nunca ocurre. Primero comprueba el SHA-256 como se explica arriba; cuando coincida, elige «Más información» y luego «Ejecutar de todas formas». Si no puedes comprobarlo, o Windows informa de malware y no solo de un editor desconocido, no lo ejecutes.

## La ventana de Windows no se abre

Al hacer clic en Obituary en el menú Inicio o en la bandeja no pasa nada, o te avisa de que falta un componente de ejecución. La ventana se dibuja con el runtime de Microsoft Edge WebView2. Windows 11 lo incluye y la mayoría de los equipos con Windows 10 ya lo tienen, pero donde falta, la ventana no puede arrancar. Descarga e instala el runtime de WebView2 desde Microsoft (la versión Evergreen) y vuelve a hacer clic en Obituary; cuando se abra la ventana, está resuelto.

De momento la ventana existe solo para x64. El archivo de Windows para ARM64 no incluye `obit-gui.exe`, así que allí usas la línea de comandos. Si «Open Obituary» en el menú de la bandeja muestra una notificación diciendo que no encuentra el programa de la ventana, es que `obit-gui.exe` no está junto a `obitd.exe`, que es justo cómo viene el archivo para ARM64. En x64, pon los dos en la misma carpeta.

## macOS: la app dice que no es compatible con esta versión

La ventana de macOS de la 1.6.0 (`Obituary.app` dentro del DMG) necesita macOS 26. En macOS 15 y anteriores, al abrirla aparece un mensaje que dice que la app no es compatible con esta versión de macOS, y no arranca. Es un error de empaquetado de la 1.6.0, corregido en la 1.6.1. Hasta entonces la ventana necesita macOS 26. El problema está solo en la ventana que va dentro del DMG. Cuando tengas la 1.6.1 o posterior, la app se abre, y así lo sabrás.

## macOS: «no se puede verificar el desarrollador»

La app del DMG no tiene firma de Developer ID ni está notarizada, así que Gatekeeper la detiene la primera vez. Después de comprobar el SHA-256, ábrela normalmente una vez. Cuando macOS diga que no puede verificar al desarrollador, ve a Ajustes del Sistema, luego Privacidad y seguridad, busca Obituary al final de la página, elige «Abrir igualmente» y confirma de nuevo. Los pasos están en [Primer inicio de una app sin firma](./desktop#primer-inicio-de-una-app-sin-firma). No desactives Gatekeeper de forma global para esto. Si macOS dice que la app está dañada o contiene malware, es otra cosa y no debes permitirla.

## La bandeja no arranca al iniciar sesión (macOS, Ubuntu)

Tras iniciar sesión no hay icono de Obituary en la barra de menús ni en la bandeja. El arranque al iniciar sesión depende de una entrada de inicio: en macOS, el LaunchAgent de usuario `site.woodfish.obituary.tray`; en Ubuntu, `~/.config/autostart/obituary.desktop`. Si el interruptor de Settings de la ventana está desactivado, o se movió la app, o el interruptor se activó mientras la app se ejecutaba directamente desde el DMG, la ruta guardada en la entrada ha quedado obsoleta y al iniciar sesión no se encuentra nada.

Abre la ventana y, en Settings, desactiva y vuelve a activar «Start the tray at sign-in» para que Obituary reescriba la entrada. En macOS, `launchctl list | grep obituary` debería mostrar ahora `site.woodfish.obituary.tray`; en Ubuntu, `ls ~/.config/autostart/obituary.desktop` debería encontrar el archivo, y la bandeja aparecerá en tu próximo inicio de sesión.

## Ubuntu: la ventana no se abre, o no hay icono en la bandeja

Al hacer clic en Obituary en la lista de aplicaciones no pasa nada, lo que suele significar que falta GTK 3 o WebKitGTK 4.1. `apt install ./….deb` los instala por ti; solo faltan si se eliminaron después o si la instalación se interrumpió a medias. En Ubuntu 24.04 el paquete de GTK 3 se llama `libgtk-3-0t64`; el nombre sin `t64`, `libgtk-3-0`, no existe en esta versión, así que un comando copiado de una guía antigua falla:

```bash
sudo apt update
sudo apt install libgtk-3-0t64 libwebkit2gtk-4.1-0 xdg-utils
```

Vuelve a abrir Obituary desde la lista de aplicaciones; cuando aparezca la ventana, está resuelto. Si la ventana se abre pero no hay icono en la bandeja, es porque la bandeja necesita que el escritorio ofrezca un host de StatusNotifierItem, y GNOME no lo hace de forma predeterminada. Puedes abrir la misma ventana desde la lista de aplicaciones sin ninguna extensión; para tener el icono, añade a GNOME una extensión que lo admita, como AppIndicator.

## Lo que ha caducado no se está enterrando

La vida útil de un archivo se agotó hace tiempo y sigue donde estaba, y la línea `runner:` de `obit daemon status` dice `off`. De la caducidad se encarga una tarea programada que se ejecuta cada 5 minutos: «Obituary Runner» en el Programador de tareas de Windows, un agente de usuario de launchd en macOS, el temporizador de usuario de systemd `obituary.timer` en Linux. Si no está instalada, se desactivó o se instaló para otro directorio de datos, nadie actúa cuando llega la hora; en ese caso, lo caducado espera hasta el siguiente `obit sweep`, o hasta el hook de un agente, para que se ocupen de ello.

Ejecuta `obit daemon enable` para instalar la tarea (en Windows también instala la bandeja al iniciar sesión), y `obit sweep` si quieres la limpieza ya. En un Linux sin servicio de usuario de systemd, como en algunos contenedores y sistemas mínimos, `obit daemon enable` no puede instalarla; usa cron para ejecutar `obitd --once --home <directorio de datos> --config-dir <directorio de configuración>` cada 5 minutos, y `obit config` te dice ambos directorios. Cuando la línea `runner` de `obit daemon status` deje de decir `off`, está resuelto; en Linux también puedes ver el temporizador con `systemctl --user list-timers | grep obituary`.

## Algo caducó pero no se enterró

El elemento aparece en la página «For you» de la ventana, en «Burials that failed», con un motivo al lado. A menudo un archivo está abierto en un editor, en un cliente de sincronización o en otro programa, y sigue donde estaba. Enterrar es mover el archivo, y eso falla cuando está en uso o te faltan permisos. Obituary no lo fuerza: el original queda intacto, lo reintenta pasado un rato, esperando más cada vez, y tras varios fallos se detiene y espera a que decidas.

Cierra lo que tiene el archivo abierto y haz clic en «Retry», o ejecuta `obit sweep`. Si ya no quieres que se ocupe de él, haz clic en «Stop tracking»: el archivo se queda donde está y la cuenta atrás termina. Cuando el elemento desaparece de «Burials that failed» y aparece en `obit graveyard`, se ha enterrado.

## Un archivo resucitado tiene otro nombre

`obit resurrect` no dio ningún error, pero el archivo volvió como `report (resurrected).txt`, porque en el sitio original ya había algo con ese nombre. Resucitar nunca sobrescribe, así que, cuando el nombre está ocupado, devuelve el archivo a la misma carpeta con otro nombre, y las dos copias quedan intactas. Compáralas, quédate con la que quieras y cámbiale el nombre.

## El archivo de configuración tiene un error

`obit config` dice `config file has errors`, seguido de una ruta, un número de línea y el nombre de una clave. `config.toml` contiene una clave que Obituary no conoce o un valor que no se admite. Obituary no lo ignora en silencio, porque entonces `retention = 30` dejaría vigente la retención de 7 días mientras tú creías que había cambiado. Corrige la ortografía, las comillas o la duración en la línea que indica y vuelve a ejecutar `obit config`; cuando deje de quejarse y la línea `config` de `obit doctor` sea `PASS`, está resuelto. Cómo se escribe cada ajuste está en la [referencia de configuración](./config).

## Un hook no tiene efecto

Tras `obit init`, un agente pudo borrar un archivo sin que nada se lo impidiera, o parece que no ocurre nada. Hay tres causas habituales: no se reinició el agente y sigue con su configuración anterior; la integración de este agente se instaló en otro ámbito (global o de proyecto); o en el entorno del agente está definida `OBIT_DISABLED=1` (desactiva todos los hooks) o `OBIT_GUARD_RM=0` (desactiva la guardia de eliminación).

Primero mira qué hay instalado:

```bash
obit init -g --show        # lo instalado de forma global
obit init --show           # lo instalado en el proyecto actual
obit doctor --verbose      # cómo está conectado cada agente
```

Si no está instalado, instálalo con `obit init -g --agent <nombre>`, y añade `--dry-run` si quieres ver antes el plan. Reinicia después el agente; en Codex, ejecuta también `/hooks` y confía en los hooks de Obituary. Para ver qué recibe realmente el hook, define `OBIT_HOOK_AUDIT=1` en la terminal desde la que arrancas el agente, usa el agente un rato y luego ejecuta `obit hook audit`. Los comandos de borrado denegados no necesitan ese interruptor, porque siempre se registran. Los errores del propio hook van a `hook.log` en el directorio de datos, y la línea `hook-log` de `obit doctor` cuenta los errores de las últimas 24 horas.

Cuando la línea del agente en `obit doctor` (por ejemplo `claude`) es `PASS` y dice `integration is current`, está instalado; si usas la guardia de eliminación, una línea como `claude-guard-rm` debería decir `registered: yes`. Ejecuta `obit doctor` en la misma terminal desde la que arrancas el agente: si una variable de entorno ha desactivado los hooks, esa línea pasa a `WARN` y nombra la variable.

## No se puede aprobar la propuesta de un contenedor Docker

«Approve» en la propuesta de un contenedor aparece atenuado en la ventana, y la tarjeta dice «Docker is not running: start Docker to decide on this proposal.». Registrar un contenedor significa preguntarle a Docker su ID completo y su fecha de creación, y eso no se puede hacer con Docker apagado. Inicia Docker Desktop o Docker Engine y vuelve a aprobar; cuando se pueda hacer clic en «Approve» y `obit docker list` muestre el contenedor, ha funcionado.

Cuando un contenedor caduca, Obituary solo lo detiene y conserva su capa de escritura, y resucitarlo lo vuelve a iniciar; un contenedor detenido se elimina únicamente después del periodo de retención. Las imágenes, los volúmenes y las redes no se tocan nunca.

## Falla una actualización, o las descargas desde China continental son lentas

`obit update --check` da un error, o el script de instalación va lento o falla; lo más probable es que este equipo no llegue a GitHub. Cuando GitHub tarda en responder, `obit update --check` y el script de instalación cambian por su cuenta al espejo de China continental. Para usar el espejo desde el principio, ejecuta `bash install.sh --mirror cn` en macOS y Linux, o `.\install.ps1 -Mirror cn` en Windows. Si descargas a mano, toma el archivo y `checksums.txt` de la misma versión y de la misma fuente. Cuando `obit update --check` imprima una línea `Latest:`, está resuelto.
