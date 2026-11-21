# 설정 참고

Obituary의 설정은 설정 디렉터리에 있는 `config.toml`에 씁니다. 이 파일은 필수가 아닙니다. 파일이 없거나 몇 줄만 적혀 있어도 나머지는 모두 기본값으로 동작합니다. `obit config`를 실행하면 파일의 위치, 파일이 있는지 여부, 각 항목이 지금 실제로 갖고 있는 값을 알려 줍니다. `obit config --create`는 모든 값이 기본값인 주석 달린 파일을 만들어 주므로, 여기서부터 고쳐 나가는 것이 가장 편합니다.

설정 디렉터리는 Windows에서는 `%APPDATA%\obituary`, macOS에서는 `~/Library/Application Support/obituary`, Linux에서는 `~/.config/obituary`입니다(`XDG_CONFIG_HOME`을 지정했다면 그 아래의 `obituary`). 다른 곳에 두려면 환경 변수 `OBIT_CONFIG_DIR`을 지정합니다.

| 키 | 기본값 | 사용할 수 있는 값 |
| --- | --- | --- |
| `graveyard.mode` | `graveyard` | `graveyard`, `recycle` |
| `graveyard.retention_days` | `7` | 1에서 3650 사이의 정수 |
| `proposals.mode` | `off` | `off`, `tiered`, `auto`, `ask` |
| `hooks.session_cleanup` | `true` | `true`, `false` |
| `hooks.session_summary` | `false` | `true`, `false` |
| `watch.stable_window` | `10m` | 0보다 큰 시간. 예: `30s`, `10m`, `1h30m` |
| `explorer.enabled` | `true` | `true`, `false` |
| `explorer.everywhere` | `false` | `true`, `false` |
| `explorer.in_git` | `false` | `true`, `false` |
| `ui.language` | `auto` | `auto`, `en`, `zh-CN` |
| `ui.tray` | `true` | `true`, `false` |
| `ui.taskbar` | `true` | `true`, `false` |
| `update.check` | `true` | `true`, `false` |
| `telemetry.enabled` | `true` | `true`, `false` |
| `agent_shells` | 지정 안 됨 | 테이블. 키는 에이전트 이름, 값은 `bash` 또는 `powershell` |
| `codex_windows_shell` | 지정 안 됨 | `bash`, `powershell` |

## 만료된 항목이 가는 곳

기본적으로 수명이 다한 항목은 Obituary의 묘지로 들어갑니다. `graveyard.retention_days`일 동안(기본 7일)은 `obit resurrect`로 원래 자리에 되돌릴 수 있고, 그 기간이 지나야 영구적으로 삭제됩니다. 이 일수는 1에서 3650 사이에서 바꿀 수 있습니다.

`graveyard.mode`를 `recycle`로 바꾸면 만료된 항목은 시스템 휴지통으로 넘어가고, 그 뒤는 Windows가 관리합니다. 이 모드는 Windows에만 있으며 macOS와 Linux에는 묘지만 있습니다. 휴지통에 들어간 항목은 휴지통에서 복원해야 하고, `obit resurrect`로는 찾을 수 없습니다.

## 자동 등록과 에이전트 세션

`proposals.mode`는 Obituary가 스스로 파일에 수명을 붙일지를 정합니다. 기본값인 `off`는 자동으로 아무것도 하지 않고, 에이전트나 사용자가 명령, 컨텍스트 메뉴, 폴더 정책으로 등록한 것만 들어옵니다. `tiered`에서는 에이전트가 이번 세션에서 만든 파일과 세션이 시작한 컨테이너가 바로 등록되고 나중에 알려 주며, 원하지 않으면 `obit spare`로 되돌릴 수 있습니다. 나머지는 `obit proposals`에서 승인을 기다립니다. `auto`는 자동 등록이 찾아낸 모든 것을 바로 등록하고 나중에 알려 줍니다. `ask`는 모든 항목을 먼저 묻습니다. 어느 모드에서든 Git이 이미 추적하는 파일, 그리고 의존성, 빌드 결과물, 캐시(`node_modules`, `dist`, `__pycache__` 등)는 자동으로 등록되지 않습니다. 규칙은 [등록 대상](./auto-register)에 있습니다.

`hooks.session_cleanup`은 기본으로 켜져 있습니다. 각 세션이 끝나 갈 무렵 에이전트는 이번 세션에서 만든 파일 중 수명이 없고 Git도 추적하지 않는 파일을 안내받고, 임시 파일에 수명을 붙이거나(`obit mark`) 매장하라는 요청을 받습니다. 결과물은 그대로 남습니다. `hooks.session_summary`는 기본으로 꺼져 있습니다. 켜면 세션이 시작될 때 프로젝트에서 24시간 안에 만료되는 항목과 최근 매장된 항목도 에이전트에게 알려 줍니다. 꺼 둔 것은 토큰을 아끼기 위해서이며, 에이전트가 매장된 경로를 건드리면 어차피 알려 주기 때문입니다.

## 폴더 정책과 탐색기의 날짜

`watch.stable_window`는 폴더 정책의 인내심입니다. 새 파일이 나타난 뒤 이만큼 변화가 없어야 옮겨지므로, 아직 내려받는 중인 파일이 치워지지 않습니다. Go의 시간 형식을 쓰며, 숫자 뒤에 `s`, `m`, `h`를 붙이고 `1h30m`처럼 이어 쓸 수 있습니다. 0보다 커야 합니다.

`explorer.*`는 Windows에서만 의미가 있습니다. Obituary는 탐색기가 파일 이름 옆에 매장 날짜를 보여 주게 하며, 실제 파일 이름은 바뀌지 않습니다. 기본적으로는 다운로드, 바탕 화면(하위 폴더 포함), 폴더 정책이 관리하는 위치에서만 보입니다. `explorer.everywhere`를 켜면 추적 중인 다른 위치에서도 보이고, Git 작업 영역은 `explorer.in_git`을 `true`로 하지 않는 한 제외됩니다. `explorer.enabled`를 `false`로 하면 아예 기록하지 않습니다. [폴더 정책](./folder-policies)을 참고하세요.

## 인터페이스

`ui.language`는 명령줄, 작은 창, 트레이, 컨텍스트 메뉴의 언어를 정합니다. 지금은 `en`과 `zh-CN`만 있습니다. `auto`는 환경 변수 `OBIT_LANG`, 그다음 `LC_ALL`, `LC_MESSAGES`, `LANG`, 마지막으로 시스템 표시 언어를 차례로 봅니다. `ui.tray`는 트레이 아이콘(macOS에서는 메뉴 막대 아이콘)을 보이거나 숨깁니다. 숨겨도 백그라운드 정리와 업데이트 확인은 계속됩니다. `ui.taskbar`는 Windows에서만 적용되며, 작은 창이 열려 있는 동안 작업 표시줄에 버튼을 둘지를 정합니다.

## 업데이트 확인과 사용 통계

`update.check`가 켜져 있으면 Obituary는 GitHub의 공개 릴리스 정보를 하루에 최대 한 번 읽으며, 사용 데이터는 보내지 않습니다. 꺼도 `obit update --check`로 언제든 직접 확인할 수 있습니다. `telemetry.enabled`는 익명 사용 통계를 제어합니다. 업데이트 확인과는 별도의 스위치이며, `obit config telemetry off`는 통계만 끄고 이 컴퓨터의 설치 ID와 전송 대기 중인 데이터를 지웁니다. 자세한 내용은 [개인정보 처리방침](./privacy)에 있습니다.

## Windows에서 에이전트가 쓰는 셸

삭제 가드는 에이전트가 실행하려는 삭제 명령을 읽어 낼 수 있어야 그에 해당하는 `obit` 명령을 안내할 수 있습니다. Windows의 에이전트는 bash를 쓸 수도, PowerShell을 쓸 수도 있고 Obituary는 이를 구별하지 못하므로, 사용자가 선언합니다. 이 에이전트는 이 Windows 컴퓨터에서 이 셸로 명령을 실행한다는 선언입니다. 선언이 없으면 의미가 모호한 명령은 차단되지 않고 통과합니다.

```toml
[agent_shells]
codex = "powershell"
```

`agent_shells`의 키는 에이전트 이름입니다. 현재 이 값을 읽는 것은 `codex`, `codebuddy`, `codebuddy-ide`, `minimax-code`입니다. `codex_windows_shell`은 Codex에만 쓰던 예전 표기이며, `agent_shells.codex`와 함께 다른 값으로 적으면 설정이 거부됩니다.

## 잘못 썼을 때

Obituary는 모르는 키를 조용히 무시하지 않습니다. 무시한다면 `retention = 30`이라고 써도 7일 보존 기간이 그대로 적용되는데, 사용자는 30일로 바뀐 줄 알게 됩니다. 그래서 `obit config`는 알아보지 못한 키에 대해 파일 경로, 줄 번호, 키의 전체 이름을 알려 주고, 철자가 비슷하면 올바른 키를 제안합니다.

```text
obit: config file has errors: …\config.toml:2: unknown key "graveyard.retention" (did you mean "graveyard.retention_days"?)
```

값이 범위를 벗어났을 때도 같은 방식으로 어느 항목이고 무엇이 허용되는지 알려 줍니다. 예를 들면 `graveyard.retention_days must be between 1 and 3650`입니다. TOML 문법 오류도 `obit config`를 실패하게 합니다. 파일을 고친 뒤 다시 실행해 더 이상 오류가 나오지 않으면 해결된 것입니다.

## 자주 쓰는 명령

```powershell
obit config                  # 파일 위치, 데이터 디렉터리, 현재 적용 중인 값
obit config --create         # 주석 달린 기본 파일 만들기
obit config --language en    # 표시 언어 저장: auto, en, zh-CN
obit config telemetry off    # 사용 통계 끄기(on이면 다시 켜기)
```

`--create`는 기존 파일을 덮어쓰지 않습니다. 파일이 이미 있으면 오류를 알리고 끝납니다. `--language`는 파일이 아직 없으면 기본 내용과 함께 파일을 만듭니다.
