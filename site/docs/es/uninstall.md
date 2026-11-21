# Desinstalación

```powershell
obit uninstall --dry-run      # see what would be removed
obit uninstall                # remove everything Obituary set up, after you confirm
.\install.ps1 -Uninstall      # if you used the install script: also remove the program files, PATH entry and Start menu shortcut
```

`obit uninstall` retira solo lo que añadió Obituary: los hooks de los agentes, MCP y la Skill instalados para el usuario y para el proyecto Git actual, el inicio automático al iniciar sesión, la tarea programada, el demonio en ejecución, el menú contextual, las fechas del Explorador y las políticas de carpetas.

## Se conservan el cementerio y el registro

De forma predeterminada se conservan el cementerio y el registro. El comando indica dónde está el cementerio y cuánto contiene. Si de verdad quieres eliminar ambos, añade `--purge-data`: primero verás el contenido del cementerio y se te pedirá confirmación otra vez.

## Hooks en otros proyectos

Si instalaste hooks a nivel de proyecto en otros proyectos, ejecuta `obit uninstall` en cada uno antes de quitar el programa. De lo contrario, esos hooks dejarán de funcionar cuando ya no encuentren `obit`.

## Scoop y winget

Ejecuta primero `obit uninstall` y después el comando de desinstalación del gestor de paquetes que usaste.
## Alcance de la desinstalación y datos

`obit uninstall --dry-run` muestra lo que puede quitarse del usuario y del proyecto Git actuales. La ejecución elimina hooks, MCP, Skills, bandeja, inicio automático y programadores, pero no cambia la instalación de otros proyectos; repite el comando allí.

| Elemento | Windows | macOS | Linux |
| --- | --- | --- | --- |
| Programa | `obit`, `obitd`, `obit-gui` opcional | `/Applications/Obituary.app` o directorio CLI | Directorio CLI; escritorio en `.deb` |
| Hook, MCP, Skill | Configuración del agent | Configuración del agent | Configuración del agent |
| Programación | Tarea, proceso residente y bandeja | LaunchAgent y barra de menús | Temporizador systemd y XDG autostart |
| Proyecto | Partes añadidas a `.claude/` y `.codex/` | Configuración del proyecto | Configuración del proyecto |

Los datos predeterminados están en `%LOCALAPPDATA%\obituary` y `%APPDATA%\obituary` en Windows, `~/Library/Application Support/obituary` en macOS y `$XDG_DATA_HOME/obituary` en Linux (`~/.local/share/obituary` si no existe). `OBIT_HOME`, `OBIT_CONFIG_DIR` y `OBIT_USER_HOME` redirigen datos, configuración y hogar; también puede existir `.obituary` en el mismo volumen.

El cementerio, el registro, el historial y la configuración se conservan. `--purge-data` solo elimina datos del cementerio cuya propiedad puede verificarse y conserva archivos desconocidos. En modo recycle se restaura desde la papelera, no con `resurrect`.
