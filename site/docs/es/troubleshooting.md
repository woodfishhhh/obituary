# Solución de problemas

Empieza con `obit doctor`, que revisa la instalación, las integraciones, los servicios y los datos sin modificar archivos. Añade `--json` o `--verbose` para obtener más detalles.

## Instalación y comprobaciones

Si no se encuentra `obit`, revisa `PATH` y abre un terminal nuevo. Verifica siempre el archivo de la misma versión:

```bash
sha256sum --check --strict checksums.txt
```

En PowerShell compara `Get-FileHash .\obit-*.zip -Algorithm SHA256` con `checksums.txt` y no ejecutes un archivo que no coincida.

## Escritorio y servicios

SmartScreen y Gatekeeper pueden bloquear paquetes sin firma. Comprueba SHA-256 y permite el primer inicio solo desde la configuración de seguridad del sistema. Windows requiere WebView2; Ubuntu 24.04 requiere GTK 3 y WebKitGTK 4.1:

```bash
sudo apt update
sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0
```

Comprueba los servicios con `obit daemon status`, `launchctl list | grep obituary`, `systemctl --user list-timers | grep obituary` y `crontab -l`.

## Archivos, integraciones y Docker

Un archivo ocupado permanece en su sitio y se vuelve a intentar; cierra el programa que lo usa y ejecuta `obit sweep`. Un TOML incorrecto aparece con archivo y línea en `obit config`. Si un hook no funciona, ejecuta `obit init --show`, `obit init --dry-run` y `obit doctor --verbose`, y revisa que no esté definido `OBIT_DISABLED=1`.

Inicia Docker antes de aprobar propuestas de contenedores. Si el destino de una restauración ya existe, `obit resurrect` no sobrescribe: usa un nombre sin conflicto.

Después de cambiar el nombre de una unidad montada por WSL, confirma el camino real de Windows y no fuerces una restauración con el camino antiguo. Para el espejo chino usa `bash install.sh --mirror cn` y no mezcles archivos de versiones distintas.
