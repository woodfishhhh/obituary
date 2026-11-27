# Inicio rápido

Obituary da a los archivos y carpetas una vida útil. Cuando se agota, se entierran en un cementerio; puedes resucitarlos en cualquier momento durante el periodo de retención (7 días de forma predeterminada), y solo después se eliminan de verdad. Está diseñado para agentes de IA: los scripts de depuración, registros, capturas de pantalla y resultados de pruebas que produce un agente tienen una vida útil desde el momento en que se crean. Nadie tiene que acordarse de limpiar, y el `rm -rf` de un agente ya no destruye nada para siempre.

## 1. Instalar

Windows 10 / 11, macOS y Linux (x64 y ARM64):

| | Windows | macOS | Linux |
|---|---|---|---|
| Instalar con | Scoop, el script `install.ps1` (winget en revisión) | Homebrew, el script `install.sh` | el script `install.sh` |
| Comprobación de caducidad | proceso residente de la bandeja, Programador de tareas como alternativa | agente launchd del usuario | temporizador `systemd --user`; crontab de usuario si no hay systemd |
| CLI, cementerio y resurrección, registro automático (opcional), guardia de eliminación, integración con agentes (hooks, Skill, MCP), contenedores Docker, políticas de carpetas | sí | sí | sí |
| Bandeja, ventana pequeña, menú Inicio y accesos directos del escritorio | sí | DMG universal de macOS: [ventana y barra de menús](./desktop) | Ubuntu 24.04 amd64: [ventana y bandeja](./desktop) |
| Menú contextual y fechas de entierro en el gestor de archivos | Explorador | Finder: Acciones rápidas, Etiquetas de fecha | Nautilus: Scripts · Dolphin: menú, Etiquetas de fecha |
| Modo de papelera de reciclaje | sí | solo el cementerio | sí |

**Lo más fácil: deja que lo haga tu agente.** Envía esta frase al agente que utilizas (Claude Code, Codex, Cursor, etc.). Instalará Obituary, se conectará a sí mismo y se detendrá cuando la decisión te corresponda.

```text
Read https://obituary.woodfish.site/agent-install.md and follow it to install Obituary and connect it to yourself.
```

**A mano:** elige tu sistema operativo a continuación y ejecuta `obit doctor` para comprobar la instalación.

### macOS / Linux

En macOS, Homebrew lo instala en una sola línea:

```bash
brew install --cask woodfishhhh/tap/obituary
```

También puedes descargar y leer el script de instalación y después ejecutarlo (macOS y Linux):

```bash
curl -fsSL https://raw.githubusercontent.com/woodfishhhh/obituary/main/install.sh -o install.sh
bash install.sh
# Custom directory: --install-dir "$HOME/bin"; preview only: --dry-run
```

El script instala la última versión de forma predeterminada (con `--version <version>` puedes elegir otra). El archivo se descarga en cuatro partes a la vez (algunas rutas limitan la velocidad de cada conexión) y vuelve a una sola conexión si eso no funciona. Detecta el sistema operativo y la arquitectura, verifica SHA-256 desde la misma fuente de descarga e instala `obit` y `obitd` en `~/.local/bin`. Sigue las instrucciones para PATH y ejecuta `obit --version`. Usa `--mirror cn` para el espejo de China continental; su script está en `https://cn.obituary.woodfish.site/install.sh`.

La primera vez que uses Obituary en macOS, ejecuta `obit --version` y `obitd --help`. Si Gatekeeper solicita autorización, abre Ajustes del Sistema → Privacidad y seguridad, elige «Abrir igualmente» para cada ejecutable cuyo origen hayas verificado y vuelve a ejecutar el comando.

En WSL, guarda los proyectos y la configuración del agente en el sistema de archivos nativo de Linux, por ejemplo en `~/projects`. Los montajes DrvFS como `/mnt/c` pueden no admitir los cambios de nombre seguros que requiere la instalación. Usa solo la edición de Windows para asignar una vida útil a sus archivos, enterrarlos o restaurarlos.

En Docker, si overlayfs no proporciona identificadores persistentes, `obit mark` rechaza el registro y deja el archivo en su sitio. Para poder restaurar los archivos cuando caduquen, guarda tanto los archivos como el directorio de datos de Obituary en un volumen o montaje vinculado respaldado por un sistema de archivos como ext4 que proporcione esos identificadores.

### Windows

::: code-group

```powershell [Scoop]
scoop bucket add obituary https://github.com/woodfishhhh/scoop-bucket
scoop install obituary
```

```powershell [winget]
# In review, available once approved
winget install woodfishhhh.Obituary
```

```powershell [Install script]
# Install script: download it, read it, then run it
# (installs to %LOCALAPPDATA%\Programs\obituary, adds it to your user PATH, creates a Start menu shortcut)
irm https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1 -OutFile install.ps1
.\install.ps1          # installs the latest release; -WhatIf previews, -NoPath leaves PATH alone
```

:::

- Los tres descargan el mismo archivo desde la versión publicada en GitHub y comprueban su SHA-256 antes de descomprimirlo; una discrepancia detiene la instalación.
- Firma de código: [SignPath.io](https://about.signpath.io/) proporciona la firma de código gratuita y [SignPath Foundation](https://signpath.org/) proporciona el certificado. Consulta la [política de firma de código](code-signing) y la [política de privacidad](privacy).
- Obituary todavía no tiene firma de código. Si descargas el archivo con un navegador, Windows puede mostrar «Windows protegió tu PC»; elige «Más información → Ejecutar de todas formas». Scoop, winget y el script de instalación normalmente no provocan este aviso.
- La ventana pequeña `obit-gui.exe` necesita el runtime de WebView2 (integrado en Windows 11 y presente en la mayoría de los equipos con Windows 10) y solo está disponible para x64; la compilación ARM64 incluye la línea de comandos y la bandeja.
Después, comprueba la instalación:

```powershell
obit doctor              # health check: short summary, full HTML report (--open, --verbose, --json)
```

## 2. Conectar agentes

```powershell
obit init -g                 # Claude Code, all projects; also the Skill (and desktop integration on Windows)
obit init -g --agent codex   # Codex
```

Reinicia el agente después. Para otros agentes, el servidor MCP y Agent Skill, consulta [Conectar agentes](./agents).

En macOS / Linux, Claude Code usa `~/.claude`, Codex usa `~/.codex` y las Skills compartidas usan `~/.agents/skills`. También se respetan las anulaciones `CLAUDE_CONFIG_DIR` y `CODEX_HOME`.

Con la supervisión de escritorio activada, el proceso de la bandeja atiende los vencimientos y una tarea independiente comprueba los pendientes cada hora. Sin supervisión de la bandeja para ese directorio de datos, la tarea se ejecuta cada cinco minutos.

```bash
obit daemon enable --dry-run
obit daemon enable
obit doctor
```

macOS usa el launchd del usuario que ha iniciado sesión; Linux usa `systemd --user`. En Linux sin systemd, programa `obitd --once` cada 5 minutos en el crontab del usuario y proporciona las rutas absolutas de `obitd`, el directorio de datos `--home` y el directorio de configuración `--config-dir`. Los hooks del agente también inician una comprobación en segundo plano cuando llega el momento de trabajar.

## 3. Comandos comunes

Sustituye `<id>` por el número de un recibo de `obit add` o `obit bury`, o por el número que aparece en `obit list` / `obit graveyard`.

```powershell
obit add debug.log 2h        # bury it in 2 hours (alias: mark)
obit list                    # what is left and for how long
obit extend <id> 1d          # one more day
obit bury old-build          # bury right away
obit graveyard               # what is in the graveyard and when it will be purged
obit resurrect <id>          # put it back, never overwriting
obit resurrect --last        # undo the last deletion
```

Todo lo demás está en [Comandos](./commands).

## Siguiente

| Para saberlo | Leer |
|---|---|
| Cómo funcionan el registro, el entierro, la resurrección y la eliminación definitiva, y dónde está el cementerio | [Cómo funciona](./how-it-works) |
| Cómo conectar Claude Code, Codex y otros agentes | [Conectar agentes](./agents) |
| Qué se incluye: registros de agentes, menú contextual y registro automático opcional | [Qué se incluye](./auto-register) |
| Convertir el `rm -rf` de un agente en un entierro que puedas deshacer | [Guardia de eliminación](./delete-guard) |
| Qué ocurre cuando un agente accede a un archivo enterrado | [Avisos del cementerio](./grave-notices) |
| Vidas útiles para carpetas como Descargas y Escritorio | [Políticas de carpetas](./folder-policies) |
| Usar Obituary sin la línea de comandos | [Ventana, bandeja y menú contextual](./desktop) |
| Contenedores Docker iniciados en una sesión | [Contenedores Docker](./docker) |
| Todos los comandos | [Comandos](./commands) |
| Eliminar Obituary | [Desinstalar](./uninstall) |
| Cualquier otra cosa | [Preguntas frecuentes](./faq) |
