# Conectar agentes

## Claude Code

```powershell
obit init -g                         # all projects (~/.claude); obit init alone installs into this project (.claude/)
obit init -g --show --agent claude   # what is installed
obit init -g --uninstall             # remove only what Obituary added
```

`obit init` registra `obit hook claude` en `settings.json` para SessionStart, antes de cada llamada a una herramienta, después de las escrituras, en Stop y en SessionEnd. Crea `OBIT.md` para el agente y añade una línea de referencia a `CLAUDE.md`: `@OBIT.md` en `~/.claude/CLAUDE.md` para el usuario (`-g`), o `@.claude/OBIT.md` en el `CLAUDE.md` del proyecto para una instalación local.

- Se conservan tus ajustes y el orden de las claves. Volver a ejecutar el comando no duplica entradas; si has movido `obit.exe`, actualiza su ruta.
- Cada edición está precedida por una copia de seguridad: `.bak` la primera vez, luego `.bak.1`, `.bak.2` …; las antiguas copias de seguridad nunca se sobrescriben.
- Si un hook falla, el agente continúa sin interrupción y el error se anota en `hook.log`. Con `OBIT_DISABLED=1` puedes desactivar los hooks temporalmente.

## Codex

```powershell
obit init -g --agent codex   # ~/.codex/hooks.json plus a block in AGENTS.md; obit init --agent codex installs into this project
```

- **Codex solo ejecuta los hooks que has marcado como de confianza.** Después de la instalación, abre Codex, ejecuta `/hooks` y marca los hooks de Obituary como de confianza. Los hooks de proyecto también requieren que confíes en el propio proyecto.
- Windows y WSL usan el mismo `hooks.json`. Las rutas `/mnt/<drive>/…` de WSL se convierten en `<drive>:\…`. Por ahora se omiten las del sistema de archivos propio de WSL (`/home/…`).
- Codex edita los archivos con `apply_patch`. Obituary lee el parche para identificar los archivos añadidos, modificados o movidos; los eliminados no cuentan.

## Otros agentes

```powershell
obit init -g --show                         # every supported agent, how far each integration goes, what is installed
obit init -g --agent gemini,cursor          # install one or several; without -g, into this project only
obit init -g --agent gemini --mcp --skill   # also register the MCP server and install the Skill (where supported)
obit init -g --agent gemini --uninstall     # remove only what Obituary added
```

Si omites `--agent`, solo se configura Claude Code. Obituary nunca se instala por su cuenta en otros agentes.

| Método | Agentes |
|---|---|
| Hooks nativos | GitHub Copilot (VS Code y Copilot CLI), Cursor, Gemini CLI, Factory Droid, Trae / Trae CN, Kimi, Mistral Vibe, Google Antigravity; Tencent CodeBuddy (CLI e IDE), Qoder / Qoder CN / Lingma, Zhipu ZCode |
| Plugin (Obituary instala un pequeño complemento que invoca `obit hook`) | OpenCode, OpenClaw, Pi, Oh My Pi, Hermes, Cline, Kilo Code, MiniMax Code, DeepSeek Harness |
| Archivo de reglas y MCP solamente | Roo Code, Windsurf, WorkBuddy, QoderWork |

::: tip Funciones disponibles por agente
No todos los agentes admiten el registro después de una escritura, el aviso al iniciar sesión, el guardia de eliminación, MCP y la Skill. La [tabla de compatibilidad](https://github.com/woodfishhhh/obituary/blob/main/docs/agents.md) del repositorio indica qué funciones están disponibles, cuáles tienen limitaciones y cuáles no se admiten, y explica los motivos.
:::

Si la documentación pública no basta para modificar la configuración con seguridad, `obit init` muestra todos los pasos manuales e indica que la instalación está incompleta y requiere intervención. Nunca da por instalado algo que no ha instalado.

## Servidor MCP y Agent Skill

```powershell
obit init -g --mcp --skill                 # Claude Code; add --agent codex for Codex. Neither is installed by default
```

- **`obit mcp`** es un servidor MCP que usa la entrada y la salida estándar (stdio). El agente puede invocar `obit_mark`, `obit_mktemp`, `obit_bury`, `obit_resurrect`, `obit_extend`, `obit_spare` y las herramientas de consulta `obit_list`, `obit_graveyard`, `obit_proposals` y `obit_stats`.
- **Las decisiones son tuyas.** No hay herramientas MCP para aprobar o rechazar propuestas, confiar en `.obitrules`, eliminar definitivamente elementos del cementerio ni cambiar la configuración. El agente solo puede trasladarte la propuesta.
- La **Agent Skill** es un archivo `SKILL.md` que explica cuándo asignar una vida útil a lo que produce el agente, cómo eliminarlo de forma recuperable y qué debes decidir tú. Si lo editas, Obituary deja de sobrescribirlo o eliminarlo.
- Reinicia el agente después de instalar. `obit doctor` comprueba si ambos componentes están instalados y apuntan al `obit.exe` actual.
