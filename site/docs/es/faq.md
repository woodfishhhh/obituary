# Preguntas frecuentes

## ¿Por qué no eliminar directamente?

Si eliminas un archivo y luego resulta que lo necesitabas, no puedes recuperarlo. Obituary lo entierra en el cementerio y lo conserva siete días de forma predeterminada. Durante ese plazo, `obit resurrect` lo devuelve a su sitio; la eliminación definitiva solo ocurre después. Si el agente intenta usarlo en esos siete días, recibe en ese momento el número de la tumba y el comando para resucitarlo (véase [Avisos del cementerio](./grave-notices)).

## ¿Funciona en macOS y Linux?

Sí. Ambos cuentan con programas de línea de comandos y en segundo plano para x64 y ARM64, con cementerio, resurrección y entierro automático. macOS usa launchd y Linux un temporizador de usuario de systemd para comprobar los vencimientos. Los hooks del agente también inician comprobaciones cuando hace falta. Consulta el [inicio rápido](./index). El modo de papelera, la ventana, la bandeja del sistema y el menú contextual son exclusivos de Windows.

## ¿Qué hago si Windows dice que protegió mi equipo?

Obituary aún no tiene firma de código. Si descargas el archivo comprimido con un navegador y aparece ese aviso, elige «Más información → Ejecutar de todas formas». Scoop, winget y el script de instalación normalmente no provocan el aviso; los tres verifican primero el SHA-256 del archivo.

## ¿Hay una ventana en ARM64?

No. `obit-gui.exe` solo está disponible para x64. La versión ARM64 incluye la línea de comandos y la bandeja del sistema.

## ¿Podría registrarse código que todavía no he incluido en un commit?

No. El registro automático está desactivado por defecto: solo entran los elementos que registra el agente con `obit mark` y los que añades desde el menú contextual. Incluso si lo activas, solo afecta a elementos sin seguimiento dentro de un árbol de trabajo de Git. Los archivos nuevos que no coinciden con ninguna regla (como código que aún no has incluido en un commit) no se listan. Las dependencias, los resultados de compilación y las cachés se dejan intactos. Lo registrado por una regla o propuesta no se entierra si ya está en un commit al caducar. Consulta [Qué se registra](./auto-register).

## ¿Qué pasa si Git falla o si mi `.gitignore` abarca muchos archivos?

Antes de registrar nada, Obituary pregunta a Git si el elemento está bajo seguimiento. Si Git no puede responder (no está instalado, se agota el tiempo de espera o el repositorio está dañado), no se registra nada. Las carpetas fuera de repositorios Git nunca se registran automáticamente. Solo tú o el agente podéis darles una vida útil con `obit mark`. Un `.gitignore` muy amplio deja más elementos sin seguimiento, pero el registro automático sigue limitado a las reglas incluidas y al archivo `.obitrules` en el que hayas confiado.

## ¿Puede el agente hacer pasar mis archivos por suyos para evitar la propuesta?

No. Por defecto el registro automático está desactivado y nada se registra automáticamente sin tu aprobación. En modo escalonado solo se registran directamente los archivos que se puede demostrar que creó el agente durante la sesión: el hook presenció la escritura, el archivo no estaba en la instantánea inicial y su fecha de creación en el sistema de archivos es posterior al inicio. Los archivos que ya existían o que creó otro programa requieren propuesta. Aunque alguien induzca al agente a renombrar una carpeta antigua para que coincida con una regla, no cambia su fecha de creación: solo se propone. Los archivos bajo seguimiento de Git nunca se registran automáticamente, en ningún modo. Recibes un aviso por cada registro directo, puedes deshacerlo con `obit spare` y lo enterrado se puede resucitar durante siete días.

## ¿Puede el agente aprobar propuestas en mi nombre?

No. No hay herramientas MCP para aprobar o rechazar propuestas, confiar en `.obitrules`, eliminar definitivamente elementos del cementerio o cambiar ajustes, incluido el modo de propuestas. El agente solo puede trasladarte la propuesta.

## ¿Funciona `obit resurrect` en el modo de papelera?

No. Si configuras `mode = "recycle"` en `config.toml`, los elementos enterrados van a la papelera del sistema y debes restaurarlos desde allí.

## ¿Cómo puedo desactivarlo temporalmente?

- Hooks: `OBIT_DISABLED=1`
- Guardia de eliminación: `OBIT_GUARD_RM=0`
- Limpieza automática: elige «Pause» en el menú de la bandeja.

## ¿Dónde se guardan los datos?

`%LOCALAPPDATA%\obituary` contiene el registro, el log de auditoría, el log del demonio y el cementerio de esta unidad. La configuración está en `%APPDATA%\obituary\config.toml`. Consulta [Cómo funciona](./how-it-works#where-the-data-lives).

## ¿Al desinstalar se elimina el cementerio?

No. `obit uninstall` conserva el cementerio y el registro por defecto. Añade `--purge-data` solo si quieres eliminar ambos. Consulta [Desinstalación](./uninstall).
