# Справочник конфигурации

Obituary читает `config.toml` из каталога конфигурации. Все ключи необязательны; для пропущенных используются значения ниже. `obit config` показывает путь и действующие значения.

## Ключи

| Ключ | По умолчанию | Значения или формат | Действие |
| --- | --- | --- | --- |
| `codex_windows_shell` | не задан | `bash`, `powershell` | Объявляет shell Codex в Windows; неоднозначные команды удаления пропускаются, если ключ не задан. |
| `agent_shells.<agent>` | не задан | `bash`, `powershell` | Объявляет Windows shell для одного hook agent. |
| `graveyard.mode` | `graveyard` | `graveyard`, `recycle` | Перемещает истёкшие элементы на кладбище или в системную корзину. |
| `graveyard.retention_days` | `7` | Целое число от `1` до `3650` | Число дней, когда элемент можно восстановить. |
| `watch.stable_window` | `10m` | Длительность Go: `30s`, `10m`, `1h` | Ждёт стабильности перед перемещением по политике каталога. Значение должно быть положительным. |
| `explorer.enabled` | `true` | `true`, `false` | Записывает даты в проводник. |
| `explorer.everywhere` | `false` | `true`, `false` | Показывает даты во всех поддерживаемых каталогах. |
| `explorer.in_git` | `false` | `true`, `false` | Показывает даты внутри Git worktree. |
| `ui.language` | `auto` | `auto`, `en`, `zh-CN` | Язык CLI, окна и трея. |
| `ui.tray` | `true` | `true`, `false` | Включает значок в трее или строке меню. |
| `ui.taskbar` | `true` | `true`, `false` | Показывает отдельную кнопку окна Windows на панели задач. |
| `hooks.session_summary` | `false` | `true`, `false` | Показывает сводку при начале сеанса. |
| `hooks.session_cleanup` | `true` | `true`, `false` | Убирает файлы, созданные этим сеансом и оставшиеся без категории. |
| `proposals.mode` | `off` | `off`, `tiered`, `auto`, `ask` | Определяет, какие автоматические регистрации ждут подтверждения. |
| `update.check` | `true` | `true`, `false` | Автоматически проверяет стабильные выпуски. |
| `telemetry.enabled` | `true` | `true`, `false` | Разрешает статистику использования; этот переключатель независим от обновлений. |

`agent_shells` — таблица с именами агентов, например `[agent_shells] codex = "powershell"`. `codex_windows_shell` — старое имя для Codex; различающиеся значения отклоняются.

## Форматы и ошибки

Длительности используют синтаксис Go и могут сочетать `ns`, `us`, `ms`, `s`, `m`, `h`, например `1h30m`. Срок хранения должен быть от `1` до `3650` дней. `graveyard.mode`, `ui.language` и `proposals.mode` принимают только перечисленные значения.

Неизвестные ключи не игнорируются. Ошибка содержит путь, строку и полный ключ, а при близком написании предлагает вариант. Ошибка TOML также приводит к сбою `obit config`.

## Частые команды

```powershell
obit config
obit config --create
obit config --language en
obit config telemetry off
obit config telemetry on
```

`--create` записывает шаблон с комментариями только при отсутствии файла. `--language` принимает `auto`, `en` и `zh-CN`. `telemetry on|off` меняет только `telemetry.enabled` и не меняет `[update] check`.
