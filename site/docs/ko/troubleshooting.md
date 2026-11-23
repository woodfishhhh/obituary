# 문제 해결

문제가 생기면 먼저 `obit doctor`를 실행하세요. 읽기만 하고 아무것도 쓰지 않으며, 설치, 에이전트 연동, 백그라운드 스케줄, 데이터를 항목별로 점검합니다. 각 줄은 `PASS`, `INFO`, `WARN`, `FAIL` 중 하나로 시작하고, 이어서 원인과 할 일이 적혀 있습니다. `FAIL`이 있으면 명령은 0이 아닌 종료 코드로 끝납니다. 결과를 다른 사람에게 보여 주려면 `--json`을, 각 에이전트에 어떤 기능이 연결되어 있는지 보려면 `--verbose`를 붙이세요.

아래 항목은 모두 같은 순서로 씁니다. 무엇이 보이는지, 왜 그런지, 어떻게 하는지, 해결됐는지 어떻게 확인하는지입니다.

업그레이드 후 탐색기 메뉴 전체가 다른 지원 언어로 표시되더라도 메뉴는 정상 작동합니다. `obit doctor`는 정보로만 알립니다(JSON에서는 `installation: label-language-mismatch`). `obit init --context-menu --yes`를 실행하면 현재 인터페이스 언어로 바뀝니다.

## 터미널에서 `obit`을 찾지 못함

터미널이 `command not found`라고 하거나 PowerShell이 `obit`을 인식하지 못한다면, 프로그램은 디스크에 있지만 그 폴더가 PATH에 없는 것입니다. Windows 설치 스크립트는 `%LOCALAPPDATA%\Programs\obituary`를 사용자 PATH에 추가하지만, 이미 열려 있는 터미널은 새 PATH를 읽지 못합니다. `install.sh`는 프로그램을 `~/.local/bin`에 두고 PATH는 절대 수정하지 않으며, 이 폴더가 처음부터 PATH에 없는 시스템도 있습니다. Ubuntu `.deb`는 `/usr/libexec/obituary/`에 두는데, 여기도 PATH에 들어 있지 않습니다.

Windows에서는 터미널을 닫고 새로 여세요. macOS와 Linux에서는 `~/.local/bin`을 셸 설정에 추가합니다. zsh라면 `~/.zshrc`에 `export PATH="$HOME/.local/bin:$PATH"` 한 줄을 넣고 새 터미널을 여세요. `.deb`만 설치했다면 전체 경로 `/usr/libexec/obituary/obit`을 쓰거나, 설치 스크립트로 명령줄을 따로 설치하세요.

`obit --version`이 버전을 출력하고 `obit doctor`의 `path` 줄이 더 이상 `WARN`이 아니면 해결된 것입니다.

## `obit doctor`가 `obitd`를 찾지 못함

`obit doctor`에 `FAIL obitd`가 나오면 `obit` 옆에 백그라운드 프로그램 `obitd`가 없다는 뜻입니다. Obituary는 같은 폴더에서 이를 찾으므로 `obit` 하나만 복사해서는 부족합니다. `obit`과 `obitd`를 함께 두거나, 설치한 방법에 맞게 설치를 다시 하세요. `obit doctor`를 다시 실행했을 때 그 줄이 `PASS`가 되면 해결된 것입니다.

## 내려받은 파일이 체크섬을 통과하지 못함

설치 스크립트를 쓰면 이 확인은 스크립트가 직접 하며, 해시가 맞지 않으면 아무것도 설치하지 않고 멈춥니다. 압축 파일, DMG, `.deb`를 직접 내려받았을 때만 손으로 확인하면 됩니다. 맞지 않는 이유는 대개 다운로드가 덜 됐거나, 전송 중에 파일이 바뀌었거나, `checksums.txt`가 다른 버전의 것이기 때문입니다. 해시는 버전마다 계산되기 때문입니다.

같은 릴리스 페이지에서 패키지와 `checksums.txt`를 다시 내려받고(중국 본토라면 미러에서 받을 수도 있습니다) 한 번 더 확인하세요. 그래도 맞지 않으면 실행하지 마세요. 아래의 `VERSION`과 파일 이름은 실제로 내려받은 것으로 바꿉니다. Linux에서는 다음과 같습니다.

```bash
file=obituary_VERSION_linux_amd64.deb
grep -F "  $file" checksums.txt | sha256sum --check --strict -
```

macOS에서는 마지막 줄의 `sha256sum --check --strict -`를 `shasum -a 256 -c -`로 바꿉니다. Windows PowerShell에서는 다음과 같습니다.

```powershell
$file = "obituary_VERSION_windows_amd64.zip"
(Get-FileHash $file -Algorithm SHA256).Hash -eq ((Select-String -Path checksums.txt -SimpleMatch $file).Line -split '\s+')[0]
```

해시가 일치하면 Linux와 macOS는 파일 이름 뒤에 `OK`를, PowerShell은 `True`를 출력합니다.

## Windows가 ‘Windows의 PC 보호’ 화면을 띄움

브라우저로 직접 내려받은 압축 파일은 Windows SmartScreen이 막을 수 있습니다. Obituary의 프로그램에 아직 코드 서명이 없기 때문입니다. Scoop, winget, 설치 스크립트로 설치하면 대체로 마주치지 않습니다. 먼저 위의 방법으로 SHA-256을 확인하고, 일치하면 ‘추가 정보’를 누른 뒤 ‘실행’을 누르세요. 확인할 수 없거나, 시스템이 단순한 ‘알 수 없는 게시자’가 아니라 악성 코드로 보고한다면 실행하지 마세요.

## Windows 작은 창이 열리지 않음

시작 메뉴나 트레이에서 Obituary를 눌러도 반응이 없거나, 런타임이 없다는 메시지가 나옵니다. 창은 Microsoft Edge WebView2 런타임으로 그려집니다. Windows 11에는 기본으로 들어 있고 Windows 10에도 대부분 설치되어 있지만, 없는 컴퓨터에서는 창이 시작되지 않습니다. Microsoft 사이트에서 WebView2 런타임(Evergreen 버전)을 내려받아 설치한 뒤 Obituary를 다시 누르세요. 창이 열리면 해결된 것입니다.

창은 아직 x64 전용입니다. ARM64용 Windows 압축 파일에는 `obit-gui.exe`가 없으므로 그곳에서는 명령줄을 쓰세요. 트레이 메뉴의 ‘Open Obituary’가 창 프로그램을 찾을 수 없다는 알림을 띄운다면, `obit-gui.exe`가 `obitd.exe` 옆에 없기 때문이며, ARM64 압축 파일이 바로 그런 구성입니다. x64에서는 둘을 같은 폴더에 두세요.

## macOS: app이 현재 시스템 버전과 호환되지 않는다고 함

1.6.0의 macOS 작은 창(DMG 안의 `Obituary.app`)은 macOS 26이 필요합니다. macOS 15 이하에서 열면 이 app이 현재 버전과 호환되지 않는다는 메시지가 나오고 실행되지 않습니다. 1.6.0 패키징 때 생긴 문제이며 1.6.1에서 수정됩니다. 그때까지 이 창을 쓰려면 macOS 26이 필요합니다. 문제는 DMG 안의 작은 창에만 있습니다. 1.6.1 이상을 설치해 app이 열리면 해결된 것입니다.

## macOS: ‘개발자를 확인할 수 없음’

DMG 안의 app에는 Developer ID 서명이 없고 공증도 되어 있지 않아서, 처음 열 때 Gatekeeper가 막습니다. SHA-256을 확인한 뒤 먼저 평소처럼 한 번 여세요. 개발자를 확인할 수 없다는 메시지가 나오면 ‘시스템 설정 → 개인정보 보호 및 보안’에서 페이지 아래쪽의 Obituary를 찾아 ‘그래도 열기’를 누르고 한 번 더 확인합니다. 단계는 [서명되지 않은 app을 처음 열기](./desktop#서명되지-않은-app을-처음-열기)에 있습니다. 이를 위해 Gatekeeper를 전체적으로 끄지 마세요. app이 손상되었다거나 악성 코드를 포함한다고 나오는 것은 다른 문제이므로 허용하지 마세요.

## 트레이가 로그인할 때 시작되지 않음(macOS, Ubuntu)

로그인한 뒤에도 메뉴 막대나 트레이에 Obituary 아이콘이 없습니다. 로그인 시 시작은 시작 항목에 의존합니다. macOS에서는 사용자 LaunchAgent `site.woodfish.obituary.tray.supervised`, Ubuntu에서는 `obituary-tray.service`입니다. 작은 창 Settings의 스위치가 꺼져 있거나, app을 옮겼거나, DMG 안에서 직접 실행하는 동안 스위치를 켰다면, 항목에 기록된 경로가 낡아서 로그인할 때 아무것도 찾지 못합니다.

작은 창을 열고 Settings에서 ‘Start the tray at sign-in’을 껐다가 다시 켜서 Obituary가 항목을 다시 쓰게 하세요. macOS에서는 `launchctl list | grep obituary`에 `site.woodfish.obituary.tray.supervised`가 보여야 하고, Ubuntu에서는 `systemctl --user status obituary-tray.service`으로 서비스 상태를 확인하세요.

## Ubuntu: 작은 창이 열리지 않거나 트레이 아이콘이 없음

앱 목록에서 Obituary를 눌러도 반응이 없다면 대개 GTK 3나 WebKitGTK 4.1이 없는 것입니다. `apt install ./….deb`는 이를 함께 설치하므로, 나중에 지웠거나 설치가 도중에 실패했을 때만 없습니다. Ubuntu 24.04에서 GTK 3 패키지 이름은 `libgtk-3-0t64`이고, `t64`가 없는 `libgtk-3-0`은 이 릴리스에 존재하지 않으므로, 오래된 안내를 그대로 입력하면 실패합니다.

```bash
sudo apt update
sudo apt install libgtk-3-0t64 libwebkit2gtk-4.1-0 xdg-utils
```

앱 목록에서 다시 열어 창이 나타나면 해결된 것입니다. 창은 열리는데 트레이 아이콘만 없다면, 트레이는 데스크톱이 StatusNotifierItem 호스트를 제공해야 하는데 GNOME은 기본으로 제공하지 않기 때문입니다. 확장 기능 없이도 앱 목록에서 같은 창을 열 수 있으며, 아이콘이 필요하면 AppIndicator처럼 이를 지원하는 확장 기능을 GNOME에 설치하세요.

## 만료된 항목이 매장되지 않음

`obit daemon status`로 예약 작업을 확인하세요. 만료된 항목을 지금 처리하려면 `obit reap`을 실행하세요.

데스크톱에서 감독 기능을 켜면 트레이 프로세스가 만료 시점에 처리하고 별도 작업이 한 시간마다 누락된 항목을 확인합니다. 이 데이터 디렉터리에 트레이 감독이 설정되지 않으면 5분마다 확인합니다.

`obit doctor`는 트레이 감독 주체와 최근 실패 횟수, 마지막 종료 이유를 보여 줍니다. 재시작 대기, 일시 정지, 의도적인 중지, 감독 항목 누락 및 감독 프로세스 종료를 구분합니다.

`obit daemon enable`로 예약 작업을 설치하세요(Windows에서는 로그인 시 시작하는 트레이도 함께 설치됩니다). 지금 바로 정리하려면 `obit sweep`을 실행합니다. systemd 사용자 서비스가 없는 Linux(일부 컨테이너나 최소 구성 시스템)에서는 `obit daemon enable`로 설치할 수 없으므로, 대신 cron으로 5분마다 `obitd --once --home <데이터 디렉터리> --config-dir <설정 디렉터리>`를 실행하세요. 두 디렉터리는 `obit config`가 알려 줍니다. `obit daemon status`의 `runner` 줄이 더 이상 `off`가 아니면 해결된 것이며, Linux에서는 `systemctl --user list-timers | grep obituary`로도 타이머를 볼 수 있습니다.

## 만료됐지만 매장에 실패함

작은 창의 ‘For you’ 페이지 ‘Burials that failed’에 그 항목이 이유와 함께 나옵니다. 편집기, 동기화 클라이언트, 다른 프로그램이 파일을 열고 있는 경우가 많고, 파일은 제자리에 남아 있습니다. 매장은 파일을 옮기는 일이어서 사용 중이거나 권한이 없으면 실패합니다. Obituary는 억지로 하지 않습니다. 원본은 그대로 두고 잠시 뒤에 간격을 점점 늘려 가며 다시 시도하고, 실패가 반복되면 멈추고 사용자의 결정을 기다립니다.

파일을 잡고 있는 프로그램을 닫고 ‘Retry’를 누르거나 `obit sweep`을 실행하세요. 더 이상 다루지 않으려면 ‘Stop tracking’을 누릅니다. 파일은 제자리에 남고 카운트다운은 끝납니다. 항목이 ‘Burials that failed’에서 사라지고 `obit graveyard`에 나타나면 매장된 것입니다.

## 부활한 파일의 이름이 바뀜

`obit resurrect`가 오류 없이 끝났는데 돌아온 파일의 이름이 `report (resurrected).txt`입니다. 원래 자리에 같은 이름이 이미 있었기 때문입니다. 부활은 절대 덮어쓰지 않으므로, 이름이 쓰이고 있으면 같은 폴더에 다른 이름으로 되돌리며, 두 파일의 내용은 모두 온전합니다. 두 파일을 비교해 필요한 것을 남기고 이름을 고치세요.

## 설정 파일에 실수가 있음

`obit config`가 `config file has errors`라고 하며 파일 경로, 줄 번호, 키 이름을 알려 줍니다. `config.toml`에 Obituary가 모르는 키가 있거나 허용되지 않는 값이 있다는 뜻입니다. Obituary는 이를 조용히 무시하지 않습니다. 무시하면 `retention = 30`이라고 써도 7일 보존 기간이 그대로인데 사용자는 바뀐 줄 알게 되기 때문입니다. 알려 준 줄의 철자, 따옴표, 시간 형식을 고치고 `obit config`를 다시 실행하세요. 오류가 더 나오지 않고 `obit doctor`의 `config` 줄이 `PASS`이면 해결된 것입니다. 각 항목을 쓰는 방법은 [설정 참고](./config)에 있습니다.

## 훅이 동작하지 않음

`obit init` 뒤에도 에이전트가 파일을 삭제하는 것을 막지 못했거나 아무 일도 일어나지 않는 것 같습니다. 흔한 원인은 세 가지입니다. 에이전트를 재시작하지 않아 예전 설정을 쓰고 있거나, 이 에이전트의 연동이 다른 범위(전역 또는 프로젝트)에 설치되어 있거나, 에이전트 환경에 `OBIT_DISABLED=1`(모든 훅 끄기)이나 `OBIT_GUARD_RM=0`(삭제 가드 끄기)이 설정되어 있는 경우입니다.

먼저 무엇이 설치되어 있는지 봅니다.

```bash
obit init -g --show        # 전역에 설치된 것
obit init --show           # 현재 프로젝트에 설치된 것
obit doctor --verbose      # 각 에이전트의 연결 상태
```

설치되어 있지 않다면 `obit init -g --agent <이름>`으로 설치하고, 계획을 먼저 보려면 `--dry-run`을 붙입니다. 그런 다음 에이전트를 재시작하세요. Codex에서는 안에서 `/hooks`를 실행해 Obituary의 훅을 신뢰해야 합니다. 훅이 실제로 무엇을 받는지 보려면 에이전트를 시작하는 터미널에서 `OBIT_HOOK_AUDIT=1`을 설정하고, 한동안 쓴 다음 `obit hook audit`을 실행합니다. 거부된 삭제 명령은 이 스위치 없이도 항상 기록됩니다. 훅 자체의 오류는 데이터 디렉터리의 `hook.log`에 쓰이며, `obit doctor`의 `hook-log` 줄이 최근 24시간의 오류 수를 셉니다.

`obit doctor`에서 그 에이전트의 줄(예: `claude`)이 `PASS`이고 `integration is current`라고 되어 있으면 설치된 것입니다. 삭제 가드를 쓴다면 `claude-guard-rm` 같은 줄에 `registered: yes`가 나와야 합니다. 에이전트를 시작하는 것과 같은 터미널에서 `obit doctor`를 실행하세요. 환경 변수가 훅을 꺼 두었다면 그 줄이 `WARN`이 되고 변수 이름이 표시됩니다.

## Docker 컨테이너 제안을 승인할 수 없음

작은 창에서 컨테이너 제안의 ‘Approve’가 회색이고, 카드에 ‘Docker is not running: start Docker to decide on this proposal.’이라고 나옵니다. 컨테이너를 등록하려면 Docker에 전체 ID와 생성 시각을 물어봐야 하는데, Docker가 꺼져 있으면 할 수 없습니다. Docker Desktop이나 Docker Engine을 시작한 뒤 다시 승인하세요. ‘Approve’를 누를 수 있고 `obit docker list`에 그 컨테이너가 보이면 된 것입니다.

컨테이너가 만료되면 Obituary는 그것을 멈추고 쓰기 가능한 레이어를 남겨 둘 뿐이며, 부활하면 다시 시작합니다. 멈춘 컨테이너는 보존 기간이 지난 뒤에야 삭제됩니다.

## Docker 볼륨의 매장을 확인할 수 없음

삭제 전후에 Docker 이벤트를 대조합니다. 삭제를 확인할 수 없다면 검증된 아카이브를 보관하고 `obit doctor`를 확인하세요. 매장이 성공했다고 간주하면 안 됩니다. 확인이 필요한 항목마다 `docker-volume-recovery` 경고가 표시됩니다. 안내에 따라 원래 Docker 데몬에서 볼륨을 확인하세요. 장부에 기록되지 않은 아카이브에는 `obit resurrect`나 `obit purge`를 사용할 수 없습니다. [Docker 컨테이너와 볼륨](./docker)

## 업데이트가 실패하거나 중국 본토에서 다운로드가 느림

`obit update --check`가 오류를 내거나 설치 스크립트가 느리거나 실패한다면, 대개 이 컴퓨터가 GitHub에 닿지 못하는 것입니다. GitHub의 응답이 늦으면 `obit update --check`와 설치 스크립트는 스스로 중국 본토 미러로 바꿉니다. 처음부터 미러를 쓰려면 macOS와 Linux에서는 `bash install.sh --mirror cn`, Windows에서는 `.\install.ps1 -Mirror cn`을 실행하세요. 직접 내려받는다면 압축 파일과 `checksums.txt`를 같은 릴리스, 같은 출처에서 받아야 합니다. `obit update --check`가 `Latest:` 줄을 출력하면 해결된 것입니다.
