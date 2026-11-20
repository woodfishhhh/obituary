# Obituary의 작동 방식

## 파일의 수명

1. **등록**: 파일이나 폴더에 수명을 지정합니다. 에이전트는 `obit add debug.log 2h`를, 사용자는 파일 탐색기의 상황에 맞는 메뉴에서 ‘Add to Obituary’를 사용합니다. [등록 대상](./auto-register)을 참고하세요.
2. **만료**: `obit list`에서 남은 시간을 확인합니다. 만료 전에 `obit extend`로 수명을 늘리거나 `obit spare`로 추적을 중단하고 파일을 그대로 둘 수 있습니다.
3. **매장**: 수명이 다하면 묘지로 옮겨 기본 7일 동안 보관합니다. `obit graveyard`에서 확인할 수 있습니다.
4. **부활 또는 영구 삭제**: 보관 기간에는 `obit resurrect <id>`로 원래 위치에 되돌릴 수 있으며 기존 파일을 덮어쓰지 않습니다. 보관 기간이 끝난 뒤에만 영구 삭제합니다(`obit purge`).

나중에 필요할지 확신이 없다면 `obit bury`로 먼저 매장하세요. 7일 동안 원래 상태로 부활시킬 수 있습니다.

## 묘지 위치

묘지는 파일과 같은 드라이브에 있습니다. 데이터 디렉터리와 같은 드라이브에서는 그 디렉터리 안의 묘지를, 다른 드라이브에서는 루트의 숨김 디렉터리 `.obituary`을 사용합니다. 매장할 때는 이름만 바꾸므로 빠르고 일부만 옮겨지는 일이 없습니다.

보관 기간은 설정에서 변경합니다. 묘지 대신 시스템 휴지통으로 바로 보내도록 설정할 수도 있습니다. 이 경우 휴지통에서 직접 복원해야 하며 `obit resurrect`은 사용할 수 없습니다.

```toml
[graveyard]
mode = "graveyard"      # or "recycle": straight to the system Recycle Bin
retention_days = 7      # days the graveyard keeps things
```

설정 파일은 `%APPDATA%\obituary\config.toml`입니다. `obit config`은 현재 설정을 보여 주고 `obit config --create`는 주석이 달린 기본 설정 파일을 만듭니다.

## 다른 파일이 삭제되지 않는 이유

- **영구 삭제는 묘지 항목을 지울 때만 일어납니다.** 매장할 때마다 별도의 무덤 폴더를 만들고 해당 폴더의 파일 ID를 기록합니다. 영구 삭제나 부활 전에는 무덤이 원래 폴더인지, 안에 항목이 정확히 하나인지, 그 항목이 매장했던 대상과 같은지 확인합니다. 장부가 손상되거나 묘지가 링크로 바뀌어도 묘지 밖의 항목은 삭제되지 않습니다.
- **부활할 때는 원래 위치로만 돌아갑니다.** 이름이 이미 있으면 덮어쓰지 않고 옆에 `name (resurrected).ext`처럼 다른 이름으로 복원합니다. 두 파일 모두 남으며 실제 경로는 실행 결과에 표시됩니다. 삭제된 상위 폴더는 다시 만듭니다. 상위 폴더가 정션이나 심볼릭 링크로 바뀌었거나, 정션 대상이 바뀌었거나, 드라이브 문자가 다른 볼륨을 가리키면 부활을 거부하고 무덤은 그대로 둡니다.
- **이름이 아니라 파일을 식별합니다.** 각 항목은 볼륨 일련번호와 128비트 파일 ID를 저장합니다. 이름이나 위치가 바뀌어도 원래 파일을 찾으며 옛 경로를 차지한 다른 파일은 건드리지 않습니다. 삭제 직전에 다시 식별하며, 하드 링크가 둘 이상인 파일은 거부합니다.
- **보호된 경로가 있습니다.** 드라이브 루트, `C:\Windows`, Program Files, ProgramData, 사용자 프로필, 바탕 화면, 문서, 다운로드 등은 폴더 자체와 그 상위 폴더를 대상으로 삼을 수 없습니다. 리디렉션된 폴더도 마찬가지입니다. 그 안의 개별 항목에는 수명을 지정할 수 있습니다.
- **Git 저장소를 보호합니다.** 폴더의 어느 깊이에든 `.git`이 있으면 `obit add`와 `obit bury`에 `--force`가 필요합니다. 만료 때 다시 검사하며 검사를 끝내지 못하면 매장하지 않습니다.
- **확실하지 않으면 삭제하지 않습니다.** 파일이 실제로 사라졌다고 확인한 경우에만 기록을 포기합니다. 접근 거부나 볼륨 분리 등은 나중에 다시 시도합니다. 재시도 간격은 1분부터 점점 늘어나 최대 하루가 됩니다. 12번 연속 실패하면 멈추고 사용자 결정을 기다립니다. 이유는 `obit list` 및 Decisions의 ‘Burials that failed’에 표시됩니다. `obit extend <id> 1m`(창의 Retry)는 다시 시도하고 `obit spare <id>`(Stop tracking)는 항목을 제자리에 둡니다. 큰 폴더는 매장 전 최대 30초 동안 Git 저장소가 있는지 검색합니다. 끝내지 못하면 나중에 다시 시도하며 저장소가 들어 있는 폴더는 매장하지 않습니다.
- **모든 작업을 기록합니다.** 등록, 연장, 취소, 삭제를 모두 감사 로그(`obit log`)에 남깁니다.

## 백그라운드 실행

기본적으로 `obitd.exe`는 로그인 시 트레이에 상주해 만료 시각에 맞춰 초 단위로 매장합니다. 트레이 아이콘이 유일한 인터페이스입니다. 보완책으로 Windows 예약 작업이 5분마다, 그리고 로그인할 때마다 `obitd --once`를 실행합니다. 트레이가 종료됐거나 한 번도 시작하지 않았어도 만료 항목은 매장되고 보관 기간이 지난 항목은 영구 삭제됩니다. 에이전트 훅도 세션 시작과 종료 시 한 차례 처리하며 `obit reap`로 지금 실행할 수 있습니다.

```powershell
obit daemon enable              # tray at login plus the fallback scheduled task (--dry-run lists what would be installed)
obit daemon status              # current mode and the next expiry (--json)
obit daemon disable             # remove the scheduled task and login start, and stop the running daemon (--dry-run previews)
```

`list`, `graveyard`, `stats`, `log` 같은 조회 명령은 읽기 전용이며 매장을 실행하지 않습니다.

## 데이터 저장 위치 {#where-the-data-lives}

`%LOCALAPPDATA%\obituary` 아래에 `store.json`(장부), `audit.jsonl`(감사 로그), `obitd.log`(데몬 로그), `graveyard\`(이 드라이브의 묘지)가 있습니다.

`OBIT_HOME`(데이터), `OBIT_CONFIG_DIR`(설정), `OBIT_USER_HOME`(사용자 홈)로 각 위치를 바꿀 수 있습니다.
