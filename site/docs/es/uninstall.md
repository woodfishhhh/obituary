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
