# 제거

```powershell
obit uninstall --dry-run      # see what would be removed
obit uninstall                # remove everything Obituary set up, after you confirm
.\install.ps1 -Uninstall      # if you used the install script: also remove the program files, PATH entry and Start menu shortcut
```

`obit uninstall`은 Obituary가 추가한 항목만 제거합니다. 사용자 수준과 현재 Git 프로젝트에 설치한 에이전트 훅, MCP, Skill을 비롯해 로그인 시 자동 시작 설정, 예약된 작업, 실행 중인 데몬, 상황에 맞는 메뉴, 파일 탐색기의 날짜 표시, 폴더 정책이 대상입니다.

## 묘지와 장부는 기본적으로 남습니다

기본적으로 묘지와 장부를 보존하고 묘지의 위치와 보관량을 알려 줍니다. 둘 다 지우려면 `--purge-data`를 추가하세요. 먼저 묘지 내용을 보여 준 뒤 다시 확인을 요청합니다.

## 다른 프로젝트의 훅

다른 프로젝트에도 프로젝트 단위로 훅을 설치했다면 프로그램을 제거하기 전에 각 프로젝트에서 `obit uninstall`을 실행하세요. 그렇지 않으면 `obit`이 없어져 해당 훅이 작동하지 않습니다.

## Scoop과 winget

먼저 `obit uninstall`을 실행한 뒤 설치에 사용한 패키지 관리자의 제거 명령을 실행하세요.
## 제거 범위와 데이터 위치

`obit uninstall --dry-run`은 현재 사용자와 Git 프로젝트에서 제거할 항목을 보여 줍니다. 실제 제거는 hook, MCP, Skill, 트레이 시작, 자동 시작과 스케줄러를 정리하지만 다른 프로젝트의 설정은 건드리지 않습니다. 설치했던 각 프로젝트에서 실행하세요.

| 위치 또는 구성 요소 | Windows | macOS | Linux |
| --- | --- | --- | --- |
| 프로그램 | 설치 폴더의 `obit`, `obitd`, 선택적 `obit-gui` | `/Applications/Obituary.app` 또는 CLI 폴더 | CLI 폴더, 데스크톱은 `.deb` |
| hook, MCP, Skill | 사용자 또는 프로젝트 agent 설정 | agent 설정 | agent 설정 |
| 스케줄러 | 작업 스케줄러, 상주 프로세스, 트레이 | LaunchAgent와 메뉴 막대 | systemd 사용자 타이머, XDG 자동 시작 |
| 프로젝트 설정 | `.claude/`, `.codex/`의 Obituary 부분 | 프로젝트 설정 | 프로젝트 설정 |

기본 데이터 위치는 Windows `%LOCALAPPDATA%\obituary`, macOS `~/Library/Application Support/obituary`, Linux `$XDG_DATA_HOME/obituary`(기본 `~/.local/share/obituary`)입니다. `OBIT_HOME`, `OBIT_CONFIG_DIR`, `OBIT_USER_HOME`으로 각각 데이터, 설정, 사용자 홈을 바꿀 수 있으며 같은 볼륨의 `.obituary`에 묘지가 있을 수 있습니다.

묘지, 원장, 감사 로그와 설정은 기본적으로 남습니다. `--purge-data`는 소유권을 확인한 묘지 데이터만 제거하고 알 수 없는 파일은 남깁니다. recycle 모드에서는 시스템 휴지통에서 복원해야 하며 `resurrect`를 사용할 수 없습니다.
