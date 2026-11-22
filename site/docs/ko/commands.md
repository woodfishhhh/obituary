# 명령어

`<id>`에는 `obit add`나 `obit bury`의 실행 결과 또는 `obit list`, `obit graveyard` 목록에 나오는 번호를 입력합니다. 시간 단위 `s`, `m`, `h`, `d`, `w`는 `1d12h`처럼 조합할 수 있습니다.

## 수명

```powershell
obit add debug.log 2h                       # bury it in 2 hours (alias: mark)
obit add output\shots 1d12h                 # folders work too
obit add report.zip --at "2026-12-01 18:00" # at a given moment
obit list                                   # what is left and for how long
obit extend <id> 1d                         # one more day
obit spare <id>                             # stop tracking, keep the file (alias: cancel)
obit reap                                   # bury everything that is due now (alias: sweep; --dry-run previews)
```

## 묘지

```powershell
obit bury old-build                         # bury right away
obit graveyard                              # what is in the graveyard and when it will be purged (--here: this folder only)
obit resurrect <id>                         # put it back; if the name is taken, it comes back renamed, never overwriting
obit resurrect --last                       # undo the last deletion: everything one obit bury / obit rm buried comes back
obit purge                                  # purge what is past retention (--all for everything, after confirming; --dry-run previews)
obit purge <id>                             # purge this grave now (permanent; asks first, --yes outside a terminal)
```

`resurrect`와 `purge`에서는 `5`나 `#5`처럼 숫자만 지정하면 항상 ID로 해석합니다. 해당 ID가 없으면 오류를 반환하며 파일 이름으로 다시 찾지 않습니다. 이름이 숫자인 파일은 `./5` 또는 절대 경로로 지정하세요.

## 삭제 명령 래퍼

```powershell
obit rm -rf build            # rm's common options; every path is buried
obit rmdir empty-dir
obit git-clean -fdx
obit remove-item .\build     # one literal path only
```

## 규칙과 제안

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

## 폴더 정책 및 Docker

```powershell
obit watch C:\Users\you\Downloads 2w
obit watch --list
obit watch --remove C:\Users\you\Downloads
obit docker add web-1 3d
obit docker list
```

`obit mark --docker-volume cache 3d`(`add`도 가능)를 실행하면 `cache`를 3일 뒤에 매장합니다. 지금 매장하려면 `obit bury --docker-volume -- cache`를 실행하세요. [Docker 컨테이너와 볼륨](./docker)

## 에이전트 연동과 백그라운드 작업

```powershell
obit init                       # Claude Code hooks and Skill, context menu, Start menu and desktop shortcuts; the tray starts at login
obit init -g --agent codex      # Codex
obit init -g --guard-rm         # delete guard
obit init -g --mcp              # add the MCP server
obit init --no-context-menu --no-shortcuts --no-skill   # skip the context menu, shortcuts or Skill
obit init -g --uninstall --no-context-menu   # on uninstall keep the context menu and remove the rest; --no-shortcuts and --no-skill work the same way
obit init -g --show             # what is installed
obit daemon enable              # tray at login plus the fallback scheduled task
obit daemon status
obit daemon disable
```

데스크톱에서 감독 기능을 켜면 트레이 프로세스가 만료 시점에 처리하고 별도 작업이 한 시간마다 누락된 항목을 확인합니다. 이 데이터 디렉터리에 트레이 감독이 설정되지 않으면 5분마다 확인합니다.

## 조회 및 점검

```powershell
obit doctor              # read-only check (--json)
obit stats               # eulogy: how much was buried, space freed, resurrections (alias: eulogy)
obit log                 # the full record of everything done
obit config              # show the configuration; --create writes a commented default
obit config telemetry off # disable usage reporting and clear the local installation ID; update checks are separate
obit hook audit          # with OBIT_HOOK_AUDIT=1: what each hook received and did
obit uninstall           # remove everything Obituary set up (try --dry-run first)
```

`list`, `graveyard`, `proposals`, `stats`, `mark`, `bury`, `resurrect` 등 조회 및 작업 결과를 반환하는 명령은 스크립트와 에이전트를 위해 `--json`을 지원합니다. `obit rm` 같은 삭제 래퍼는 원래 명령의 출력 형식을 유지하므로 `--json`을 지원하지 않습니다.

## 환경 변수

| 변수 | 효과 |
|---|---|
| `OBIT_DISABLED=1` | 훅을 일시적으로 비활성화 |
| `OBIT_TELEMETRY=0`, `DO_NOT_TRACK=1` | 현재 프로세스에서 사용 통계 전송을 끕니다. CI에서는 전송하지 않습니다 |
| `OBIT_GUARD_RM=0` | 삭제 가드를 일시적으로 끕니다 |
| `OBIT_HOOK_AUDIT=1` | 각 훅이 받은 내용과 수행한 작업을 기록합니다 |
| `OBIT_TRUST_PROJECT_RULES=1` | CI에서 프로젝트의 `.obitrules` 파일을 신뢰합니다 |
| `OBIT_HOME`, `OBIT_CONFIG_DIR`, `OBIT_USER_HOME` | 데이터, 설정, 사용자 홈 디렉터리의 위치를 각각 변경합니다 |

<!-- generated-command-reference:start -->
## 매개변수 참고

### `obit`

Obituary (讣告), AI 에이전트를 위한 휴지통입니다. 파일에 수명을 지정하면 수명이 다했을 때 묘지에 매장되고, 묘지에서 부활시킬 수 있습니다

사용법: `obit [command]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | obit 도움말 |
| `--version` | `-v` | `false` | obit 버전 정보 |

### `obit add`

경로에 수명을 지정합니다(이미 추적 중인 경로를 다시 등록하면 만료 시각이 초기화됩니다)

별칭: `add`, `mark`
사용법: `obit add <path>... <duration> | add <path>... --at <time> [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--at` | 없음 | `""` | 지정한 현지 시각에 만료됩니다: "2006-01-02 15:04", "2006-01-02" 또는 "15:04" |
| `--docker-volume` | 없음 | `false` | 인수를 Docker 볼륨 이름으로 해석 |
| `--force` | 없음 | `false` | .git 저장소가 들어 있는 폴더를 허용합니다 |
| `--help` | `-h` | `false` | add 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |
| `--no-daemon` | 없음 | `false` | 백그라운드 데몬을 시작하지 않습니다 |
| `--note` | 없음 | `""` | 목록에 표시되는 자유 형식 메모 |

예시:

```text
obit add debug.log 2h
obit add output\shots 1d12h
obit add report.zip --at "2026-12-01 18:00"
```

### `obit approve`

제안을 승인합니다: 제안에 나열된 항목을 등록합니다

사용법: `obit approve <proposal id>... [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | approve 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |

### `obit bury`

경로를 지금 바로 묘지로 옮깁니다(obit resurrect로 복원)

사용법: `obit bury <path>... [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--docker-volume` | 없음 | `false` | 인수를 Docker 볼륨 이름으로 해석 |
| `--force` | 없음 | `false` | .git 저장소가 들어 있는 폴더를 허용합니다 |
| `--help` | `-h` | `false` | bury 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |
| `--note` | 없음 | `""` | 매장과 함께 보관되는 자유 형식 메모 |

### `obit cancel`

경로나 컨테이너 ID를 살려 줍니다: 추적을 멈추고 대상은 그대로 둡니다

별칭: `cancel`, `spare`
사용법: `obit cancel <id|path>... [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | cancel 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |

### `obit completion`

지정한 셸용 obit 자동 완성 스크립트를 생성합니다.

사용법: `obit completion [command]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | completion 도움말 |

### `obit completion bash`

bash 셸용 자동 완성 스크립트를 생성합니다.

사용법: `obit completion bash`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | bash 도움말 |
| `--no-descriptions` | 없음 | `false` | 자동 완성 설명을 끕니다 |

### `obit completion fish`

fish 셸용 자동 완성 스크립트를 생성합니다.

사용법: `obit completion fish [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | fish 도움말 |
| `--no-descriptions` | 없음 | `false` | 자동 완성 설명을 끕니다 |

### `obit completion powershell`

powershell용 자동 완성 스크립트를 생성합니다.

사용법: `obit completion powershell [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | powershell 도움말 |
| `--no-descriptions` | 없음 | `false` | 자동 완성 설명을 끕니다 |

### `obit completion zsh`

zsh 셸용 자동 완성 스크립트를 생성합니다.

사용법: `obit completion zsh [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | zsh 도움말 |
| `--no-descriptions` | 없음 | `false` | 자동 완성 설명을 끕니다 |

### `obit config`

설정 파일과 적용 중인 설정을 표시합니다(--create는 기본값을 기록합니다)

사용법: `obit config [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--create` | 없음 | `false` | 기본값으로 설정 파일을 만듭니다 |
| `--help` | `-h` | `false` | config 도움말 |
| `--language` | 없음 | `""` | 언어를 설정합니다: auto, en 또는 zh-CN |

### `obit config telemetry`

사용 통계: 무작위 설치 ID, 버전, OS, 아키텍처, 인터페이스 언어, 설치 채널을 설치 수와 버전별 사용 현황을 파악하는 데 씁니다. 파일, 경로, 명령은 전송하지 않으며 IP 주소와 User-Agent도 저장하지 않습니다. obit config telemetry off 또는 OBIT_TELEMETRY=0으로 끌 수 있습니다.

사용법: `obit config telemetry on|off [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | telemetry 도움말 |

### `obit daemon`

백그라운드 데몬(obitd)을 제어합니다

사용법: `obit daemon [command]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | daemon 도움말 |

### `obit daemon disable`

예약 작업과 상주 데몬을 제거합니다

사용법: `obit daemon disable [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--dry-run` | 없음 | `false` | 변경 내용만 출력합니다 |
| `--help` | `-h` | `false` | disable 도움말 |

### `obit daemon enable`

사용자 예약 작업을 활성화합니다(트레이 감독 시 매시간, 그 외에는 5분마다). Windows에서는 로그인 시 트레이도 활성화합니다

사용법: `obit daemon enable [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--dry-run` | 없음 | `false` | 변경 내용만 출력합니다 |
| `--help` | `-h` | `false` | enable 도움말 |
| `--resident` | 없음 | `false` | 예비용 예약 작업은 바꾸지 않고 트레이만 활성화합니다 |

### `obit daemon start`

데몬을 지금 시작합니다

사용법: `obit daemon start [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | start 도움말 |

### `obit daemon status`

데몬 상태와 다음 만료 시각을 표시합니다

사용법: `obit daemon status [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | status 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |

### `obit daemon stop`

데몬을 중지합니다

사용법: `obit daemon stop [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | stop 도움말 |

### `obit docker`

Docker 컨테이너를 변하지 않는 전체 ID와 생성 시각으로 추적합니다. 만료되면 컨테이너를 중지하고 쓰기 가능한 레이어는 유지합니다. 부활시키면 컨테이너를 시작합니다. [graveyard] retention_days가 지나면 영구 삭제가 중지된 컨테이너를 제거하며 볼륨은 제거하지 않습니다. 휴지통 모드에서도 마찬가지입니다. 이미지, 볼륨, 네트워크는 보존됩니다.

사용법: `obit docker [command]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | docker 도움말 |

### `obit docker add`

Docker 컨테이너 하나를 등록합니다. 만료되면 컨테이너를 중지합니다

사용법: `obit docker add <container name|ID> <lifespan> [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | add 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |

### `obit docker list`

등록된 Docker 컨테이너를 나열합니다

사용법: `obit docker list [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | list 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |

### `obit doctor`

아무것도 변경하지 않고 설치, 연동, 데이터를 점검합니다

사용법: `obit doctor [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | doctor 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |
| `--verbose` | 없음 | `false` | 에이전트 기능 세부 정보를 포함합니다 |

### `obit extend`

연장: 만료 시각을 뒤로 미룹니다(이미 만료 시각이 지났다면 지금부터 계산합니다)

사용법: `obit extend <id|path>... <duration> [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | extend 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |

예시:

```text
obit extend 3 1d
obit extend debug.log 30m
```

### `obit git-clean`

git clean의 드라이런으로 선택된 경로를 매장합니다

사용법: `obit git-clean [options] [--] [pathspec]... [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | git-clean 도움말 |

### `obit graveyard`

묘지에 있으며 아직 부활시킬 수 있는 항목을 나열합니다

별칭: `graveyard`, `graves`
사용법: `obit graveyard [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | graveyard 도움말 |
| `--here` | 없음 | `false` | 원래 경로가 현재 디렉터리와 그 하위에 있는 항목만 표시합니다 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |

### `obit help`

애플리케이션의 모든 명령에 대한 도움말을 표시합니다.

사용법: `obit help [command] [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help 도움말 |

### `obit hook`

에이전트 훅 진입점(에이전트가 호출하며 사람이 직접 쓰는 것이 아님)

사용법: `obit hook [command]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | hook 도움말 |

### `obit hook antigravity`

stdin으로 들어온 antigravity 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook antigravity [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | antigravity 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook audit`

훅과 삭제 가드의 감사 기록을 표시합니다

사용법: `obit hook audit [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | audit 도움말 |
| `--lines` | `-n` | `40` | 소스별 줄 수 |

### `obit hook claude`

stdin으로 들어온 claude 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook claude [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | claude 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook cline`

stdin으로 들어온 cline 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook cline [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | cline 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook codebuddy`

stdin으로 들어온 codebuddy 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook codebuddy [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | codebuddy 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook codebuddy-ide`

stdin으로 들어온 codebuddy-ide 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook codebuddy-ide [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | codebuddy-ide 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook codex`

stdin으로 들어온 codex 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook codex [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | codex 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook copilot`

stdin으로 들어온 copilot 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook copilot [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | copilot 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook cursor`

stdin으로 들어온 cursor 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook cursor [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | cursor 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook deepseek-harness`

stdin으로 들어온 deepseek-harness 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook deepseek-harness [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | deepseek-harness 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook droid`

stdin으로 들어온 droid 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook droid [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | droid 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook gemini`

stdin으로 들어온 gemini 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook gemini [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | gemini 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook hermes`

stdin으로 들어온 hermes 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook hermes [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | hermes 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook kilocode`

stdin으로 들어온 kilocode 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook kilocode [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | kilocode 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook kimi`

stdin으로 들어온 kimi 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook kimi [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | kimi 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook lingma`

stdin으로 들어온 lingma 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook lingma [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | lingma 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook minimax-code`

stdin으로 들어온 minimax-code 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook minimax-code [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | minimax-code 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook omp`

stdin으로 들어온 omp 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook omp [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | omp 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook openclaw`

stdin으로 들어온 openclaw 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook openclaw [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | openclaw 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook opencode`

stdin으로 들어온 opencode 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook opencode [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | opencode 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook pi`

stdin으로 들어온 pi 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook pi [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | pi 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook qoder`

stdin으로 들어온 qoder 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook qoder [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | qoder 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook qoder-cn`

stdin으로 들어온 qoder-cn 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook qoder-cn [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | qoder-cn 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook trae`

stdin으로 들어온 trae 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook trae [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | trae 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook trae-cn`

stdin으로 들어온 trae-cn 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook trae-cn [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | trae-cn 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook vibe`

stdin으로 들어온 vibe 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook vibe [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | vibe 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook zcode`

stdin으로 들어온 zcode 훅 이벤트 하나를 처리합니다. 오류가 나도 동작을 막지 않습니다

사용법: `obit hook zcode [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 페이로드에 이벤트 이름이 없을 때 사용할 호스트 이벤트 이름 |
| `--help` | `-h` | `false` | zcode 도움말 |
| `--notice-only` | 없음 | `false` | 삭제 가드 없이 참고용 컨텍스트만 전달합니다 |

### `obit hook finish`

세션이 남긴 파일을 계산합니다(훅이 백그라운드에서 시작)

사용법: `obit hook finish [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | finish 도움말 |
| `--project` | 없음 | `""` | 프로젝트 루트 |
| `--session` | 없음 | `""` | 에이전트 세션 ID |

### `obit init`

에이전트에 Obituary의 훅과 지침을 설치합니다(--uninstall로 제거)

사용법: `obit init [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--agent` | 없음 | `[claude]` | 연동할 에이전트(반복하거나 쉼표로 구분, 기본값: claude) |
| `--context-menu` | 없음 | `false` | 클래식 파일 탐색기 상황에 맞는 메뉴를 설치합니다(기본적으로 활성화) |
| `--dry-run` | 없음 | `false` | 변경 내용만 출력합니다 |
| `--format` | 없음 | `"text"` | --show의 출력 형식: text 또는 markdown(에이전트 지원 범위) |
| `--global` | `-g` | `false` | 이 프로젝트가 아니라 사용자 전체에 적용 |
| `--guard-rm` | 없음 | `false` | 지원되는 삭제 명령을 거부하고 복원 가능한 obit 대체 명령을 안내합니다 |
| `--help` | `-h` | `false` | init 도움말 |
| `--mcp` | 없음 | `false` | Obituary MCP stdio 서버를 등록합니다 |
| `--no-context-menu` | 없음 | `false` | 파일 탐색기 상황에 맞는 메뉴를 설치하지 않습니다 |
| `--no-shortcuts` | 없음 | `false` | 시작 메뉴와 바탕 화면 바로 가기를 만들지 않습니다 |
| `--no-skill` | 없음 | `false` | Obituary Agent Skill을 변경하지 않습니다 |
| `--show` | 없음 | `false` | 설치된 항목을 표시합니다 |
| `--skill` | 없음 | `false` | Obituary Agent Skill을 설치합니다(기본적으로 활성화) |
| `--uninstall` | 없음 | `false` | Obituary가 추가한 항목을 제거합니다 |
| `--yes` | 없음 | `false` | 묻지 않고 적용합니다 |

예시:

```text
obit init -g  # Claude Code, 모든 프로젝트용(~/.claude)
obit init  # Claude Code, 이 프로젝트만(.claude/)
obit init -g --agent codex  # Codex(~/.codex/hooks.json + AGENTS.md)
obit init -g --uninstall  # Obituary가 추가한 항목만 제거합니다
obit init -g --show  # 설치된 항목
obit init --show --format markdown > docs/agents.md
```

### `obit list`

추적 중인 경로와 Docker 컨테이너를 만료가 가까운 순서로 표시합니다

별칭: `list`, `ls`
사용법: `obit list [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | list 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |

### `obit log`

Obituary가 수행한 모든 작업의 감사 로그를 표시합니다

사용법: `obit log [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | log 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |
| `--lines` | `-n` | `20` | 표시할 이벤트 수 |

### `obit mcp`

MCP stdio로 Obituary 도구를 제공합니다

사용법: `obit mcp [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | mcp 도움말 |

### `obit mktemp`

스스로 매장되는 임시 디렉터리를 만들고 그 경로를 출력합니다

사용법: `obit mktemp <lifespan> [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | mktemp 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |
| `--name` | 없음 | `"tmp"` | 디렉터리 이름 접두사 |
| `--session` | 없음 | `""` | 디렉터리가 속한 에이전트 세션 |

예시:

```text
dir=$(obit mktemp 1d)
obit mktemp 2h --name screenshots
```

### `obit proposals`

승인을 기다리는 등록과 묻지 않고 이루어진 등록

사용법: `obit proposals [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | proposals 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |

### `obit purge`

선택한 매장 파일이나 보관 기간이 지난 매장 파일을 삭제합니다. --all과 명시적으로 지정한 대상은 보관 기간을 건너뛰므로 먼저 확인을 요청하며, 터미널이 아닌 환경에서는 --yes가 필요합니다. Docker 컨테이너는 --all이나 명시적 ID를 지정해도 [graveyard] retention_days가 지나야 삭제할 수 있습니다. 실행 중인 컨테이너는 부활한 것으로 취급합니다. Docker 컨테이너를 제거할 때 볼륨은 보존하며 강제 제거는 하지 않습니다. 휴지통 모드에서도 컨테이너는 똑같이 처리합니다.

별칭: `purge`, `cremate`
사용법: `obit purge [id|original path]... [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--all` | 없음 | `false` | 보관 기간이 지난 매장만이 아니라 모든 매장을 영구 삭제합니다 |
| `--dry-run` | 없음 | `false` | 실행될 내용만 표시합니다 |
| `--help` | `-h` | `false` | purge 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |
| `--yes` | 없음 | `false` | --all 또는 선택한 매장을 묻지 않고 영구 삭제합니다(에이전트는 먼저 사용자에게 물어야 함) |

### `obit reject`

제안을 거부합니다

사용법: `obit reject <proposal id>... [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | reject 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |

### `obit remove-item`

PowerShell 매개변수의 일부만 사용해 파일 시스템 항목을 매장합니다.

사용법: `obit remove-item [-Recurse] [-Force] [-Path|-LiteralPath] <path> [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | remove-item 도움말 |

### `obit resurrect`

덮어쓰지 않고 파일을 복원합니다. 매장된 Docker 컨테이너는 ID로 시작합니다

별칭: `resurrect`, `raise`
사용법: `obit resurrect <id|original path>... | --last [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | resurrect 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |
| `--last` | 없음 | `false` | 현재 세션이나 Git 프로젝트에서 가장 최근에 삭제한 묶음을 복원합니다 |

### `obit rm`

경로를 지금 바로 묘지로 옮깁니다(obit resurrect로 복원)

사용법: `obit rm [options] <path>... [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | rm 도움말 |

### `obit rmdir`

경로를 지금 바로 묘지로 옮깁니다(obit resurrect로 복원)

사용법: `obit rmdir [options] <path>... [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | rmdir 도움말 |

### `obit rules`

여기에 적용되는 규칙을 표시합니다: 기본 제공 규칙과, 신뢰한 경우 프로젝트의 .obitrules

사용법: `obit rules [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | rules 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |

### `obit rules apply`

규칙에 따라 경로를 등록합니다(Git으로 추적되는 파일은 등록되지 않음)

사용법: `obit rules apply <path>... [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | apply 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |
| `--session` | 없음 | `""` | 경로가 나온 에이전트 세션 |

### `obit rules check`

규칙이 경로에 대해 무엇을 등록할지 표시합니다(아무것도 변경하지 않음)

사용법: `obit rules check <path>... [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | check 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |
| `--session` | 없음 | `""` | 경로가 나온 에이전트 세션 |

### `obit stats`

Obituary가 한 일: 추적, 매장, 확보한 공간, 부활 횟수

별칭: `stats`, `eulogy`
사용법: `obit stats [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | stats 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |

### `obit sweep`

지금 수확: 백그라운드 프로세스를 기다리지 않고 만료된 항목을 모두 매장합니다

별칭: `sweep`, `reap`
사용법: `obit sweep [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--dry-run` | 없음 | `false` | 실행될 내용만 표시합니다 |
| `--help` | `-h` | `false` | sweep 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |

### `obit trust`

프로젝트의 .obitrules를 검토하고 신뢰합니다(수정하면 다시 검토해야 함)

사용법: `obit trust [rules file] [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | trust 도움말 |
| `--yes` | 없음 | `false` | 묻지 않고 신뢰합니다(규칙을 검토한 뒤에만 사용) |

### `obit uninstall`

Obituary 연동과 백그라운드 서비스를 제거합니다. 기본적으로 데이터는 남깁니다

사용법: `obit uninstall [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--dry-run` | 없음 | `false` | 제거 계획만 출력하고 파일이나 서비스는 변경하지 않습니다 |
| `--help` | `-h` | `false` | uninstall 도움말 |
| `--purge-data` | 없음 | `false` | 나열된 무덤을 영구 삭제하고 Obituary가 소유한 데이터를 제거합니다. 알 수 없는 파일은 남깁니다 |
| `--yes` | 없음 | `false` | 계획을 확인합니다(데이터 삭제는 --purge-data도 함께 지정한 경우에만) |

### `obit untrust`

프로젝트의 .obitrules 적용을 중단합니다

사용법: `obit untrust [rules file] [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | untrust 도움말 |

### `obit update`

안정 버전 업데이트를 확인하거나 설치합니다

사용법: `obit update [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--apply` | 없음 | `false` | 캐시된 최신 안정 버전을 다운로드하고 검증한 뒤 설치합니다(수동 설치는 Obituary 자체 프로그램만 교체합니다) |
| `--check` | 없음 | `false` | 지금 바로 GitHub를 확인합니다(자동 확인이 꺼져 있어도 확인) |
| `--help` | `-h` | `false` | update 도움말 |
| `--relaunch-gui` | 없음 | `false` | 업데이트 후 작은 창을 다시 엽니다(Windows) |
| `--wait-pid` | 없음 | `0` | 업데이트 전에 이 프로세스가 종료되기를 기다립니다(Windows) |

### `obit watch`

폴더의 최상위 하위 항목마다 각각 수명을 지정합니다

사용법: `obit watch <directory> <lifespan> | watch --list | watch --remove <directory> [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--age` | 없음 | `"first-seen"` | 경과 시간의 기준: first-seen 또는 modified(승인 필요) |
| `--clear-exclusion` | 없음 | `0` | 이 제외 ID를 명시적으로 해제합니다(--policy와 함께 사용) |
| `--help` | `-h` | `false` | watch 도움말 |
| `--json` | 없음 | `false` | JSON으로 출력합니다 |
| `--list` | 없음 | `false` | 폴더 정책을 나열합니다 |
| `--no-daemon` | 없음 | `false` | 백그라운드 프로세스를 시작하지 않습니다 |
| `--policy` | 없음 | `0` | 제외를 해제할 정책 ID |
| `--remove` | 없음 | `""` | 정책을 제거하고 그 정책이 등록한 수명을 취소합니다 |
| `--remove-id` | 없음 | `0` | ID로 폴더 정책을 제거합니다. 존재하지 않는 경로도 대상에 포함됩니다 |

<!-- generated-command-reference:end -->
