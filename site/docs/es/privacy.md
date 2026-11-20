# Política de privacidad

El registro, el cementerio, los archivos de registro y los ajustes de Obituary permanecen en tu equipo. Obituary no envía informes de fallos, archivos, rutas ni contenido de archivos.

## Estadísticas de uso

Están activadas de forma predeterminada y solo sirven para contar las instalaciones activas y conocer la distribución de las versiones. Durante el uso real del programa, Obituary puede enviar, como máximo una vez cada 24 horas, un identificador de instalación aleatorio, la versión, el sistema operativo, la arquitectura, el idioma de la interfaz y el canal de instalación. No envía archivos, rutas ni comandos, ni guarda direcciones IP o cabeceras User-Agent. Ejecuta `obit config telemetry off` o establece `OBIT_TELEMETRY=0` para desactivar los informes. No se envía nada en entornos de integración continua (CI) ni con `DO_NOT_TRACK=1`.

El servidor no guarda direcciones IP, agentes de usuario ni identificadores de instalación originales. Una huella calculada con una clave independiente permite vincular informes de la misma instalación, pero no identificar a una persona. Los datos de actividad diaria caducan a los 400 días. A largo plazo solo se conservan la huella y las fechas de la primera y la última observación para evitar duplicados. Los informes enviados correctamente solo se contabilizan como totales por canal. Solo el equipo de mantenimiento puede consultar el panel, previa autenticación.

Al desactivar los informes se eliminan el identificador de instalación local y cualquier informe pendiente. La búsqueda de actualizaciones tiene un ajuste independiente.

## Búsqueda de versiones nuevas

- Como máximo una vez al día, Obituary consulta los datos públicos de la última versión del repositorio en `https://api.github.com`. Si GitHub no responde, consulta `https://cn.obituary.woodfish.site/download/latest.json` en el servidor espejo de China continental.
- La solicitud solo incluye los datos habituales: tu dirección IP y una cabecera User-Agent que indica la versión de Obituary, por ejemplo `Obituary/1.0.0`.
- Puedes desactivarla en cualquier momento: desmarca «Check for updates automatically» en los ajustes de la ventana o añade lo siguiente a `%APPDATA%\obituary\config.toml`:

  ```toml
  [update]
  check = false
  ```

  Una vez desactivada, Obituary solo buscará actualizaciones cuando ejecutes `obit update --check`.

## Instalación y descargas

- Scoop, winget y el script de instalación descargan los archivos comprimidos de [GitHub Releases](https://github.com/woodfishhhh/obituary/releases). Se aplica la [declaración de privacidad de GitHub](https://docs.github.com/site-policy/privacy-policies/github-general-privacy-statement).
- Con el espejo (`-Mirror cn`), proceden de `cn.obituary.woodfish.site`. El servidor lleva un registro de acceso ordinario (hora, dirección IP y URL solicitada) para diagnosticar problemas.
- Al publicar una versión, GitHub Actions envía los archivos al espejo. Este los verifica y los sirve localmente sin reenviar tu solicitud de descarga a GitHub.

## Conexión de agentes

`obit init` solo modifica los archivos de configuración de los agentes en tu equipo y te muestra el plan antes de hacerlo. No envía nada a los proveedores de los agentes ni a ningún otro destino.

## Sitio web

- <https://obituary.woodfish.site/> no utiliza herramientas de analítica ni scripts de seguimiento. Nuestro servidor atiende a los visitantes de China continental y GitHub Pages al resto. Ambos mantienen registros de acceso ordinarios.
- Las direcciones de correo facilitadas en el sitio solo se usan para anunciar nuevas plataformas y versiones. Se guardan en nuestro propio servidor y no se comparten con terceros. Cada correo incluye un enlace para darse de baja.

## Contacto

Para consultas de privacidad, abre una [incidencia en GitHub](https://github.com/woodfishhhh/obituary/issues). Comunica los problemas de seguridad en privado siguiendo [SECURITY.md](https://github.com/woodfishhhh/obituary/blob/main/SECURITY.md).
