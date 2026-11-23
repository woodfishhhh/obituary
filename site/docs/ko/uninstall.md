# 제거

제거는 두 단계입니다. 먼저 Obituary가 설정해 둔 것을 걷어 내고, 그다음 프로그램 자체를 지웁니다. 묘지와 장부는 기본적으로 남으므로, 매장해 둔 파일은 제거 뒤에도 부활시킬 수 있습니다.

```powershell
obit uninstall --dry-run      # 무엇이 제거되는지 나열만 하고 아무것도 바꾸지 않음
obit uninstall                # 계획을 보여 주고, 확인한 뒤 실행
```

Windows에서 설치 스크립트로 설치했다면 명령 하나면 됩니다.

```powershell
.\install.ps1 -Uninstall      # obit uninstall --yes를 대신 실행한 뒤, 프로그램 파일, 스크립트가 추가한 PATH 항목, 시작 메뉴 바로 가기를 삭제
```

## 제거되는 것

`obit uninstall`이 다루는 것은 사용자 수준과 현재 Git 프로젝트에 있는 것입니다. 각 에이전트의 훅, MCP, Skill, 예약 작업(Windows는 작업 스케줄러, macOS는 launchd 사용자 에이전트, Linux는 systemd 사용자 타이머), 상주 프로세스와 트레이, 그리고 폴더 정책(정책 아래의 파일은 제자리에 남습니다)입니다. Windows에서는 로그인 시 자동 시작, 탐색기 컨텍스트 메뉴, 날짜 레이블도 제거합니다.

Obituary가 직접 쓴 것만 건드립니다. 사용자가 고친 항목이나 다른 설치가 남긴 항목은 그대로 두며, 어느 항목을 왜 남겼는지 계획에 적혀 있습니다. 터미널이 없는 환경(스크립트, 에이전트의 명령줄)에서는 `--yes`를 붙여야 합니다. 없으면 터미널이 필요하다고 알리고 멈춥니다.

대신 해 주지 않는 일이 세 가지 있습니다. 프로그램 파일은 설치한 방법에 맞게 지웁니다(아래 참고). 다른 프로젝트에 프로젝트 수준 훅을 설치했다면, 프로그램을 지우기 전에 각 프로젝트에서 `obit uninstall`을 실행하세요. 그러지 않으면 `obit`이 없어져 그쪽 훅이 동작하지 않습니다. 그리고 macOS와 Linux에서는 트레이의 자동 시작 항목이 목록에 없습니다. 제거하기 전에 작은 창의 Settings에서 ‘Start the tray at sign-in’을 끄세요. Obituary가 쓴 항목이 지워집니다. Windows에서는 트레이의 자동 시작을 `obit uninstall`이 처리하지만, `obit init`이 만든 시작 메뉴와 바탕 화면 바로 가기는 목록에 없습니다. 이것들은 `obit init --uninstall`이 지우지만, `--no-shortcuts`를 붙이면 남겨 둡니다.

## 묘지, 장부, 설정

기본적으로 남는 것은 세 가지입니다. 묘지에 있는 파일, 장부와 감사 로그, 그리고 `config.toml`입니다. `obit uninstall`은 묘지에 아직 얼마나 있는지, 어디에 있는지 알려 주고, 나중에 다시 설치하면 그대로 이어서 쓸 수 있습니다. 휴지통 모드를 쓴다면 만료된 항목은 이미 시스템 휴지통에 있습니다. 제거는 그것을 건드리지 않으며, 복원도 휴지통에서 해야 합니다. `obit resurrect`로는 닿지 않습니다.

모두 지우고 싶을 때는 `--purge-data`를 붙입니다.

```powershell
obit uninstall --purge-data
```

먼저 묘지의 모든 항목을 나열하고, 제거 계획을 확인한 뒤 한 번 더 확인을 받습니다. 확인 뒤 무엇이든 제거하기 전에 Obituary는 건드릴 것을 모두 점검합니다. 데이터 디렉터리가 Obituary의 것인지, 안의 파일을 모두 알아보는지, 묘지 폴더와 삭제할 무덤 하나하나를 확인할 수 있는지입니다. 하나라도 통과하지 못하면 거기서 멈추고 이유를 알려 주며, 묘지, 장부, 서비스는 그대로 남으므로 삭제가 도중에 끝나는 일이 없습니다. 모두 통과해야 그 파일들을 영구 삭제하고, 이어서 장부, 감사 로그, Obituary가 보관하던 그 밖의 파일을 지웁니다. 스크립트에서는 `--yes --purge-data`를 함께 쓰면 두 번의 확인을 건너뜁니다.

Obituary가 알아보는 파일만 지웁니다. 데이터 디렉터리에 다른 파일, 링크, 모르는 하위 폴더가 섞여 있으면 점검이 멈추고 어느 것인지 알려 줍니다. `config.toml`은 삭제 대상이 아니며, 장부와 같은 폴더(macOS 기본값)에 있어도 옮길 필요가 없습니다. 삭제 뒤 그 폴더에는 `config.toml`만 남습니다. 설정이 다른 위치에 있으면 데이터 디렉터리가 통째로 삭제됩니다.

## 프로그램 지우기

프로그램을 지우는 방법은 설치한 방법에 따라 다릅니다.

| 설치 방법 | 제거 방법 |
| --- | --- |
| Windows 설치 스크립트 | `.\install.ps1 -Uninstall` |
| Scoop | `obit uninstall` 후 `scoop uninstall obituary` |
| winget | `obit uninstall` 후 `winget uninstall woodfishhhh.Obituary` |
| macOS 작은 창(DMG) | `obit uninstall`을 실행하고 ‘Start the tray at sign-in’을 끈 뒤, 응용 프로그램 폴더의 Obituary.app을 휴지통으로 드래그 |
| Homebrew | `obit uninstall` 후 `brew uninstall --cask obituary` |
| macOS와 Linux의 `install.sh` | `obit uninstall` 후 `~/.local/bin`의 `obit`과 `obitd` 삭제 |
| Ubuntu 데스크톱 패키지(`.deb`) | ‘Start the tray at sign-in’을 끄고 트레이를 종료한 뒤 `sudo apt remove obituary-desktop` |

macOS app과 Ubuntu `.deb`는 각자 `obit`을 포함하지만 둘 다 PATH에는 넣지 않습니다. 명령줄을 따로 설치하지 않았다면 제거하기 전에 전체 경로로 실행하세요. app 안의 것은 `/Applications/Obituary.app/Contents/MacOS/obit`, `.deb`의 것은 `/usr/libexec/obituary/obit`입니다. `apt remove`는 그 폴더의 프로그램을 지우지만 묘지, 설정, 다른 방법으로 설치한 명령줄은 건드리지 않습니다.

## 데이터가 있는 곳

| | 데이터(장부, 묘지, 로그) | 설정(`config.toml`) |
| --- | --- | --- |
| Windows | `%LOCALAPPDATA%\obituary` | `%APPDATA%\obituary` |
| macOS | `~/Library/Application Support/obituary` | 같은 폴더 |
| Linux | `~/.local/share/obituary` | `~/.config/obituary` |

Linux에서 `XDG_DATA_HOME`이나 `XDG_CONFIG_HOME`을 지정했다면 각각 그 아래의 `obituary` 폴더입니다. 환경 변수 `OBIT_HOME`과 `OBIT_CONFIG_DIR`로 데이터와 설정을 다른 곳으로 옮길 수 있고, 어느 쪽을 쓰는지는 `obit config`가 출력합니다. 데이터 디렉터리가 있는 드라이브에서는 그 안의 `graveyard` 폴더가 묘지입니다. 다른 드라이브에서는 그 드라이브 루트의 숨김 폴더 `.obituary`가 묘지가 됩니다. 이렇게 해서 매장은 언제나 같은 드라이브 안의 이름 변경이 되고, 복사는 일어나지 않습니다.
