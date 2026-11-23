# Desinstalación

Desinstalar se hace en dos pasos: primero Obituary retira lo que configuró, y después eliminas los programas. El cementerio y el registro se conservan de forma predeterminada, así que los archivos que enterraste se pueden resucitar incluso después.

```powershell
obit uninstall --dry-run      # lista lo que se retiraría; no cambia nada
obit uninstall                # muestra el plan y lo aplica después de que confirmes
```

Si instalaste con el script en Windows, basta un comando:

```powershell
.\install.ps1 -Uninstall      # ejecuta obit uninstall --yes por ti y luego elimina los archivos del programa, la entrada de PATH que añadió y su acceso directo del menú Inicio
```

## Qué se retira

`obit uninstall` actúa sobre el nivel de usuario y sobre el proyecto Git actual: los hooks, MCP y Skill de cada agente, la tarea programada (el Programador de tareas en Windows, un agente de usuario de launchd en macOS, un temporizador de usuario de systemd en Linux), el proceso residente y la bandeja, y las políticas de carpetas (los archivos bajo una política se quedan donde están). En Windows también retira la entrada de inicio al iniciar sesión, el menú contextual del Explorador y las etiquetas de fecha del Explorador.

Solo toca lo que escribió Obituary. Una entrada que hayas editado, o una que dejó otra instalación, se conserva tal cual, y el plan dice cuál y por qué. Donde no hay terminal (un script, la línea de comandos de un agente) añade `--yes`; sin él, el comando se detiene y te dice que necesita una terminal.

Hay tres cosas que no hace por ti. Los archivos del programa se eliminan según la forma en que instalaste (ver más abajo). Si instalaste hooks a nivel de proyecto en otros proyectos, ejecuta `obit uninstall` en cada uno de ellos antes de eliminar los programas, o los hooks de allí fallarán porque `obit` ya no existe. Y en macOS y Linux la entrada de inicio de la bandeja no está en la lista: antes de desinstalar, desactiva «Start the tray at sign-in» en Settings de la ventana, lo que borra la entrada que escribió Obituary. En Windows `obit uninstall` se ocupa del inicio de la bandeja, pero los accesos directos del menú Inicio y del escritorio que creó `obit init` no están en su lista; `obit init --uninstall` los elimina, salvo que añadas `--no-shortcuts`.

## El cementerio, el registro y tu configuración

Se conservan tres cosas de forma predeterminada: los archivos del cementerio, el registro con el historial de auditoría y `config.toml`. `obit uninstall` te dice cuánto queda en el cementerio y dónde está, y una reinstalación posterior los recupera tal cual. Si usas el modo Papelera de reciclaje, lo que caduca ya está en la Papelera del sistema; desinstalar no la toca, y se restaura desde allí porque `obit resurrect` no llega hasta ella.

Cuando de verdad quieras borrarlo todo, añade `--purge-data`:

```powershell
obit uninstall --purge-data
```

Primero lista cada elemento del cementerio y te pide una confirmación más después de que confirmes el plan de desinstalación. Una vez confirmado, y antes de retirar nada, Obituary comprueba todo lo que va a tocar: que el directorio de datos es suyo, que reconoce cada archivo que contiene y que puede verificar la carpeta del cementerio y cada tumba que se va a purgar. Si alguna comprobación falla, se detiene ahí y te dice cuál; el cementerio, el registro y los servicios quedan como estaban, así que una purga nunca queda a medias. Solo cuando todo pasa elimina esos archivos de forma permanente, y después el registro, el historial de auditoría y los demás archivos que guarda Obituary. En un script, `--yes --purge-data` juntos se saltan las dos preguntas.

Solo elimina archivos que reconoce. Otros archivos, enlaces o una subcarpeta que Obituary no conoce hacen que la comprobación se detenga, y te dice cuál ha encontrado. `config.toml` nunca forma parte de la purga y no hace falta moverlo, ni siquiera cuando está en la misma carpeta que el registro (lo habitual en macOS): tras la purga, esa carpeta solo contiene `config.toml`. Cuando la configuración está en otro sitio, el directorio de datos se elimina entero.

## Eliminar los programas

Cómo eliminas los programas depende de cómo los instalaste.

| Instalado con | Se elimina con |
| --- | --- |
| El script de instalación de Windows | `.\install.ps1 -Uninstall` |
| Scoop | `obit uninstall` y luego `scoop uninstall obituary` |
| winget | `obit uninstall` y luego `winget uninstall woodfishhhh.Obituary` |
| La ventana de macOS (DMG) | `obit uninstall` y desactiva «Start the tray at sign-in»; después arrastra Obituary.app de Aplicaciones a la Papelera |
| Homebrew | `obit uninstall` y luego `brew uninstall --cask obituary` |
| `install.sh` en macOS o Linux | `obit uninstall` y luego borra `obit` y `obitd` de `~/.local/bin` |
| El paquete de escritorio de Ubuntu (`.deb`) | desactiva «Start the tray at sign-in», sal de la bandeja y ejecuta `sudo apt remove obituary-desktop` |

La app de macOS y el `.deb` de Ubuntu traen cada uno su propio `obit`, pero ninguno lo pone en tu PATH. Si no instalaste la línea de comandos por separado, ejecútalo con su ruta completa antes de desinstalar: `/Applications/Obituary.app/Contents/MacOS/obit` en la app, `/usr/libexec/obituary/obit` en el `.deb`. `apt remove` borra los programas de esa carpeta y deja tu cementerio, tus ajustes y cualquier línea de comandos que instalaras de otra forma.

## Dónde están los datos

| | Datos (registro, cementerio, logs) | Configuración (`config.toml`) |
| --- | --- | --- |
| Windows | `%LOCALAPPDATA%\obituary` | `%APPDATA%\obituary` |
| macOS | `~/Library/Application Support/obituary` | la misma carpeta |
| Linux | `~/.local/share/obituary` | `~/.config/obituary` |

En Linux, si `XDG_DATA_HOME` o `XDG_CONFIG_HOME` están definidas, cada una es la carpeta `obituary` que cuelga de ella. Las variables de entorno `OBIT_HOME` y `OBIT_CONFIG_DIR` llevan los datos y la configuración a otro lugar, y `obit config` imprime cuáles se están usando. En la unidad que contiene el directorio de datos, el cementerio es su carpeta `graveyard`; en las demás unidades es una carpeta oculta `.obituary` en la raíz de la unidad, de modo que enterrar siempre sea un cambio de nombre dentro de una misma unidad y nunca una copia.
