# Políticas de carpetas

Una política de carpetas asigna su propia vida útil a cada elemento que llega a la carpeta. Un caso habitual es Descargas:

```powershell
obit watch C:\Users\you\Downloads 2w        # files that land in Downloads are buried two weeks later
obit watch --list                           # folder policies, their exemptions and quiet periods
obit watch --remove C:\Users\you\Downloads  # remove a policy and the lifespans it registered
```

En macOS y Linux, tras aprobar la política, cada elemento de primer nivel tiene su propio plazo, que por defecto empieza al detectarlo por primera vez. La carpeta vigilada sigue protegida; las subcarpetas caducadas solo generan propuestas y se omiten los archivos temporales de descarga.

Por defecto, un archivo normal debe permanecer sin cambios diez minutos más después de caducar. Si cambia su tamaño o su fecha de modificación, si está abierto o si no se puede inspeccionar un proceso, el plazo vuelve a empezar. Windows bloquea la escritura durante el traslado. En macOS y Linux un proceso puede abrir el archivo entre la comprobación y el cambio de nombre.

Para aprobar cada archivo, establece `file_expiry = "propose"` en `[watch]`. Revisa las propuestas con `obit proposals` y aprueba con `obit approve <ID>`. Las subcarpetas caducadas siguen generando propuestas y se quedan en su sitio incluso tras la aprobación en macOS y Linux.

```sh
obit watch "$HOME/Downloads" 2w
```

## Cómo se aplica

- **La política necesita tu aprobación.** `obit watch` crea una propuesta. La política entra en vigor cuando ejecutas `obit approve` o la apruebas en la página «Para ti» de la ventana.
- **El archivo debe llevar diez minutos sin cambios.** Solo se entierra si ha caducado y nadie lo ha modificado en los últimos diez minutos.
- **Las subcarpetas requieren una propuesta aparte.** Cuando caduca una subcarpeta, solo se crea la propuesta. No se entierra hasta que la apruebas.
- Al quitar una política se cancelan las vidas útiles que esa política registró.

## Fechas en el Explorador de archivos {#dates-in-explorer}

En Descargas, en el Escritorio y en las carpetas con política, el Explorador muestra los elementos en seguimiento como «name (11-13 burial)». El nombre real del archivo no cambia. La etiqueta sigue el idioma de Obituary: si la interfaz está en chino simplificado, se muestra «name（11-13 下葬）»; en los demás casos, se usa la forma inglesa.

- Las fechas proceden del archivo oculto `desktop.ini` de la carpeta.
- En la sección `[explorer]` de `config.toml`, usa `enabled = false` para ocultar las fechas o `everywhere = true` para mostrarlas en todas partes.
- El Explorador guarda en caché las vistas de carpeta. Al ampliar la vida útil, la misma ventana puede seguir mostrando la fecha anterior hasta un minuto.
