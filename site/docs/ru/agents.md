# Подключение агентов

## Claude Code

```powershell
obit init -g                         # all projects (~/.claude); obit init alone installs into this project (.claude/)
obit init -g --show --agent claude   # what is installed
obit init -g --uninstall             # remove only what Obituary added
```

`obit init` регистрирует `obit hook claude` в `settings.json` для событий SessionStart, перед каждым вызовом инструмента, после записи файлов, при Stop и SessionEnd. Команда создаёт для агента файл `OBIT.md` и добавляет в `CLAUDE.md` одну строку со ссылкой: `@OBIT.md` в `~/.claude/CLAUDE.md` для текущего пользователя (`-g`) или `@.claude/OBIT.md` в `CLAUDE.md` проекта для локальной установки.

- Настройки и порядок ключей сохраняются. Повторный запуск не создаёт дубликатов. Если вы переместили `obit.exe`, повторный запуск обновит путь.
- Перед каждым изменением создаётся резервная копия: сначала `.bak`, затем `.bak.1`, `.bak.2` и так далее. Старые копии не перезаписываются.
- Если в хуке произошла ошибка, агент продолжает работу без уведомления, а ошибка записывается в `hook.log`. Параметр `OBIT_DISABLED=1` временно отключает хуки.

## Codex

```powershell
obit init -g --agent codex   # ~/.codex/hooks.json plus a block in AGENTS.md; obit init --agent codex installs into this project
```

- **Codex запускает только те хуки, которым вы явно доверяете.** После установки откройте Codex, выполните `/hooks` и добавьте хуки Obituary в доверенные. Для локальных хуков сам проект тоже должен быть доверенным.
- Windows и WSL используют общий файл `hooks.json`. Пути WSL вида `/mnt/<drive>/…` преобразуются в `<drive>:\…`. Пути в собственной файловой системе WSL (`/home/…`) пока пропускаются.
- Codex редактирует файлы с помощью `apply_patch`. Obituary узнаёт из патча, какие файлы были добавлены, изменены или перемещены. Удалённые файлы не учитываются.

## Другие агенты

```powershell
obit init -g --show                         # every supported agent, how far each integration goes, what is installed
obit init -g --agent gemini,cursor          # install one or several; without -g, into this project only
obit init -g --agent gemini --mcp --skill   # also register the MCP server and install the Skill (where supported)
obit init -g --agent gemini --uninstall     # remove only what Obituary added
```

Без `--agent` настраивается только Claude Code. Obituary не подключается к другим агентам самостоятельно.

| Метод | Агенты |
|---|---|
| Встроенные хуки | GitHub Copilot (VS Code и Copilot CLI), Cursor, Gemini CLI, Factory Droid, Trae / Trae CN, Kimi, Mistral Vibe, Google Antigravity; Tencent CodeBuddy (CLI и IDE), Qoder / Qoder CN / Lingma, Zhipu ZCode |
| Плагин (Obituary устанавливает небольшой плагин, вызывающий `obit hook`) | OpenCode, OpenClaw, Pi, Oh My Pi, Hermes, Cline, Kilo Code, MiniMax Code, DeepSeek Harness |
| Только файл правил и MCP | Roo Code, Windsurf, WorkBuddy, QoderWork |

::: tip Что поддерживает каждый агент
Регистрация после записи, уведомления в начале сеанса, защита удаления, MCP и Skill поддерживаются не всеми агентами. В [таблице совместимости](https://github.com/woodfishhhh/obituary/blob/main/docs/agents.md) для каждой возможности указаны статус («поддерживается», «с ограничениями», «не поддерживается») и причина.
:::

Если открытой документации недостаточно для безопасного изменения настроек, `obit init` выводит полную инструкцию и сообщает, что установка не завершена и нужны ручные действия. Неустановленные компоненты никогда не помечаются как установленные.

## MCP-сервер и Agent Skill

```powershell
obit init -g --mcp --skill                 # Claude Code; add --agent codex for Codex. Neither is installed by default
```

- **`obit mcp`** — MCP-сервер, который работает через стандартные потоки ввода и вывода (stdio). Агенту доступны `obit_mark`, `obit_mktemp`, `obit_bury`, `obit_resurrect`, `obit_extend`, `obit_spare` и инструменты для запросов: `obit_list`, `obit_graveyard`, `obit_proposals`, `obit_stats`.
- **Решения остаются за вами.** Инструментов MCP для одобрения или отклонения предложений, доверия к `.obitrules`, безвозвратного удаления объектов с кладбища или изменения настроек нет. Агент может только передать предложение вам.
- **Agent Skill** — файл `SKILL.md`, который объясняет агенту, когда задавать срок жизни создаваемым файлам, как удалять их с возможностью восстановления и какие решения оставить вам. После ваших изменений Obituary больше не перезаписывает и не удаляет этот файл.
- После установки перезапустите агента. `obit doctor` проверяет, установлены ли оба компонента и указывают ли они на текущий `obit.exe`.
