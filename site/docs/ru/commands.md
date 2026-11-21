# Команды

Вместо `<id>` подставьте номер из ответа `obit add` или `obit bury` либо найдите его в списке `obit list` или `obit graveyard`. Единицы времени `s`, `m`, `h`, `d` и `w` можно сочетать, например `1d12h`.

## Срок жизни

```powershell
obit add debug.log 2h                       # bury it in 2 hours (alias: mark)
obit add output\shots 1d12h                 # folders work too
obit add report.zip --at "2026-12-01 18:00" # at a given moment
obit list                                   # what is left and for how long
obit extend <id> 1d                         # one more day
obit spare <id>                             # stop tracking, keep the file (alias: cancel)
obit reap                                   # bury everything that is due now (alias: sweep; --dry-run previews)
```

## Кладбище

```powershell
obit bury old-build                         # bury right away
obit graveyard                              # what is in the graveyard and when it will be purged (--here: this folder only)
obit resurrect <id>                         # put it back; if the name is taken, it comes back renamed, never overwriting
obit resurrect --last                       # undo the last deletion: everything one obit bury / obit rm buried comes back
obit purge                                  # purge what is past retention (--all for everything, after confirming; --dry-run previews)
obit purge <id>                             # purge this grave now (permanent; asks first, --yes outside a terminal)
```

В `resurrect` и `purge` число без пути (`5` или `#5`) всегда означает ID. Если такой ID не найден, команда вернёт ошибку и не станет искать файл с таким именем. Для файла с числовым именем укажите `./5` или абсолютный путь.

## Обёртки для команд удаления

```powershell
obit rm -rf build            # rm's common options; every path is buried
obit rmdir empty-dir
obit git-clean -fdx
obit remove-item .\build     # one literal path only
```

## Правила и предложения

```powershell
obit rules                       # the rules in effect here
obit rules check debug.log out/  # preview what would be registered and by which rule
obit rules apply debug.log       # register by rule
obit trust                       # trust this project's .obitrules (--yes when non-interactive)
obit proposals                   # proposals waiting for you, and what was registered without asking
obit approve 3                   # approve
obit reject 3                    # reject
$dir = obit mktemp 1d            # a temporary folder that buries itself
```

## Правила для папок и Docker

```powershell
obit watch C:\Users\you\Downloads 2w
obit watch --list
obit watch --remove C:\Users\you\Downloads
obit docker add web-1 3d
obit docker list
```

## Интеграция и фоновая работа

```powershell
obit init                       # Claude Code hooks and Skill, context menu, Start menu and desktop shortcuts; the tray starts at login
obit init -g --agent codex      # Codex
obit init -g --guard-rm         # delete guard
obit init -g --mcp              # add the MCP server
obit init --no-context-menu --no-shortcuts --no-skill   # skip the context menu, shortcuts or Skill
obit init -g --show             # what is installed
obit daemon enable              # tray at login plus the fallback scheduled task
obit daemon status
obit daemon disable
```

## Запросы и проверки

```powershell
obit doctor              # read-only check (--json)
obit stats               # eulogy: how much was buried, space freed, resurrections (alias: eulogy)
obit log                 # the full record of everything done
obit config              # show the configuration; --create writes a commented default
obit config telemetry off # disable usage reporting and clear the local installation ID; update checks are separate
obit hook audit          # with OBIT_HOOK_AUDIT=1: what each hook received and did
obit uninstall           # remove everything Obituary set up (try --dry-run first)
```

Команды, возвращающие состояние или результат операции (`list`, `graveyard`, `proposals`, `stats`, `mark`, `bury`, `resurrect` и другие), поддерживают `--json` для скриптов и агентов. Обёртки для удаления, например `obit rm`, сохраняют формат вывода исходной команды и не поддерживают `--json`.

## Переменные окружения

| Переменная | Эффект |
|---|---|
| `OBIT_DISABLED=1` | Временно отключить хуки |
| `OBIT_TELEMETRY=0`, `DO_NOT_TRACK=1` | Отключить отправку статистики для этого процесса; в CI статистика не отправляется |
| `OBIT_GUARD_RM=0` | Временно отключить защиту удаления |
| `OBIT_HOOK_AUDIT=1` | Записывать, что получил и сделал каждый хук |
| `OBIT_TRUST_PROJECT_RULES=1` | доверять `.obitrules` проекта в CI |
| `OBIT_HOME`, `OBIT_CONFIG_DIR`, `OBIT_USER_HOME` | Изменить расположение каталогов данных, конфигурации и домашнего каталога пользователя |



## Совместимые параметры оболочек удаления

- git-clean принимает параметры Git clean -d, -f, -n, -q, -x, -X, -e pattern, --exclude pattern и --, а также --dry-run, --force и --quiet.
- rm принимает -r, -R, -f, -d, -v, --recursive, --force, --dir, --verbose, --preserve-root и -- с путями.
- rmdir принимает только -v или --verbose и пути; rmdir -p отклоняется.
- Windows remove-item принимает только -Recurse, -Force, -Path или -LiteralPath и ровно один буквальный путь. Для масок в Bash используйте obit rm.

<!-- generated-command-reference:start -->
<!-- generated-command-reference:end -->
