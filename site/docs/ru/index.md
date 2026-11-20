# Быстрый старт

Obituary задаёт срок жизни файлам и папкам. Когда срок заканчивается, они хоронятся на кладбище; в течение периода хранения (по умолчанию 7 дней) их можно в любой момент воскресить, и только после этого они действительно удаляются. Obituary создан для AI-агентов: отладочные скрипты, журналы, скриншоты и результаты тестов, созданные агентом, получают срок жизни сразу после создания. Никому не нужно помнить об уборке, а `rm -rf` агента больше не уничтожает ничего навсегда.

## 1. Установка

Windows 10 / 11, macOS и Linux (x64 и ARM64):

| | Windows | macOS | Linux |
|---|---|---|---|
| Установка с помощью | Scoop, скрипта `install.ps1` (пакет winget ещё проверяется) | Homebrew, скрипта `install.sh` | скрипта `install.sh` |
| Проверка истечения срока каждые 5 минут | фоновый процесс в области уведомлений, планировщик заданий как запасной вариант | пользовательский агент launchd | таймер `systemd --user`; crontab пользователя без systemd |
| CLI, кладбище и воскрешение, автоматическая регистрация (необязательно), защита удаления, интеграция с агентами (хуки, Skill, MCP), контейнеры Docker, политики папок | да | да | да |
| Область уведомлений, небольшое окно, контекстные меню, даты захоронения в Проводнике, меню «Пуск» и ярлыки рабочего стола | да | вместо этого CLI: `obit list`, `obit graveyard`, `obit resurrect` | как в macOS |
| Режим корзины | да | только кладбище | только кладбище |

**Проще всего поручить это агенту.** Отправьте эту фразу используемому агенту (Claude Code, Codex, Cursor и другим). Он установит Obituary, подключит его к себе и остановится там, где решение остаётся за вами.

```text
Read https://obituary.woodfish.site/agent-install.md and follow it to install Obituary and connect it to yourself.
```

**Вручную:** выберите ниже свою операционную систему и выполните `obit doctor`, чтобы проверить установку.

### macOS / Linux

На macOS Homebrew устанавливает Obituary одной строкой:

```bash
brew install --cask woodfishhhh/tap/obituary
```

Или скачайте и прочитайте скрипт установки, а затем запустите его (macOS и Linux):

```bash
curl -fsSL https://raw.githubusercontent.com/woodfishhhh/obituary/main/install.sh -o install.sh
bash install.sh
# Custom directory: --install-dir "$HOME/bin"; preview only: --dry-run
```

По умолчанию скрипт устанавливает последний релиз (`--version <version>` выбирает другой). Архив скачивается одновременно четырьмя частями (некоторые маршруты ограничивают скорость каждого соединения), а при сбое выполняется возврат к одному соединению. Скрипт определяет ОС и архитектуру, проверяет SHA-256 из того же источника загрузки и устанавливает `obit` и `obitd` в `~/.local/bin`. Следуйте инструкциям для PATH, затем выполните `obit --version`. Для материкового зеркала Китая используйте `--mirror cn`; его скрипт находится по адресу `https://cn.obituary.woodfish.site/install.sh`.

При первом запуске на macOS выполните `obit --version` и `obitd --help`. Если Gatekeeper запросит разрешение, откройте «Системные настройки → Конфиденциальность и безопасность», выберите «Всё равно открыть» для каждого исполняемого файла с проверенным источником и повторите команду.

В WSL храните проекты и конфигурацию агента в файловой системе Linux (например, в `~/projects`). Подключённые диски Windows, такие как `/mnt/c`, могут не поддерживать безопасные переименования, необходимые для установки; операция сообщит об ошибке и сохранит исходную конфигурацию. Можно также напрямую использовать версию Obituary для Windows.

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

- Все три способа скачивают один и тот же архив из релиза GitHub и проверяют его SHA-256 перед распаковкой; при несовпадении установка прекращается.
- Подпись кода: бесплатную подпись предоставляет [SignPath.io](https://about.signpath.io/), а сертификат — [SignPath Foundation](https://signpath.org/). См. [политику подписи кода](code-signing) и [политику конфиденциальности](privacy).
- Obituary пока не подписан. При скачивании архива через браузер Windows может показать сообщение «Windows защитил ваш компьютер»; выберите «Подробнее → Выполнить в любом случае». Scoop, winget и скрипт установки обычно не вызывают это сообщение.
- Небольшому окну `obit-gui.exe` нужен runtime WebView2 (он встроен в Windows 11 и есть на большинстве компьютеров с Windows 10); оно работает только на x64, а сборка ARM64 включает командную строку и область уведомлений.
Затем проверьте установку:

```powershell
obit doctor              # read-only check: PATH, obitd, runner, Claude Code / Codex integration, data directory, ledger, config (--json)
```

## 2. Подключение агентов

```powershell
obit init -g                 # Claude Code, all projects; also the Skill (and desktop integration on Windows)
obit init -g --agent codex   # Codex
```

После этого перезапустите агента. Информацию о других агентах, сервере MCP и Agent Skill см. в разделе [Подключение агентов](./agents).

В macOS / Linux Claude Code использует `~/.claude`, Codex — `~/.codex`, а общие Skills — `~/.agents/skills`. Переопределения `CLAUDE_CONFIG_DIR` и `CODEX_HOME` также учитываются.

Включите пользовательский планировщик, чтобы проверять сроки каждые 5 минут:

```bash
obit daemon enable --dry-run
obit daemon enable
obit doctor
```

macOS использует launchd вошедшего в систему пользователя, а Linux — `systemd --user`. В Linux без systemd запланируйте запуск `obitd --once` каждые 5 минут в crontab пользователя и укажите абсолютные пути к `obitd`, каталогу данных `--home` и каталогу конфигурации `--config-dir`. Хуки агента также запускают фоновую проверку, когда наступает срок работы.

## 3. Часто используемые команды

Замените `<id>` номером из квитанции `obit add` или `obit bury`, либо номером из вывода `obit list` / `obit graveyard`.

```powershell
obit add debug.log 2h        # bury it in 2 hours (alias: mark)
obit list                    # what is left and for how long
obit extend <id> 1d          # one more day
obit bury old-build          # bury right away
obit graveyard               # what is in the graveyard and when it will be purged
obit resurrect <id>          # put it back, never overwriting
obit resurrect --last        # undo the last deletion
```

Остальные команды описаны в разделе [Команды](./commands).

## Далее

| Что узнать | Читать |
|---|---|
| Как работают регистрация, захоронение, воскрешение и безвозвратное удаление и где находится кладбище | [Как это работает](./how-it-works) |
| Подключение Claude Code, Codex и других агентов | [Подключение агентов](./agents) |
| Что регистрируется: регистрации агентов, контекстное меню, необязательная автоматическая регистрация | [Что регистрируется](./auto-register) |
| Как превратить `rm -rf` агента в захоронение, которое можно отменить | [Защита удаления](./delete-guard) |
| Что происходит, когда агент обращается к захороненному файлу | [Уведомления с кладбища](./grave-notices) |
| Сроки службы папок вроде Downloads и Desktop | [Политики папок](./folder-policies) |
| Использование Obituary без командной строки | [Окно, область уведомлений и контекстное меню](./desktop) |
| Контейнеры Docker, запущенные в сеансе | [Контейнеры Docker](./docker) |
| Все команды | [Команды](./commands) |
| Удаление Obituary | [Удаление](./uninstall) |
| Всё остальное | [Часто задаваемые вопросы](./faq) |
