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

## 에이전트 연동과 백그라운드 작업

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

아래 표는 현재 소스에서 빌드한 `obit --help` 출력으로 생성됩니다.

### `obit add`
별칭: `add`, `mark`
사용법: `obit add <path>... <duration> \| add <path>... --at <time> [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--at` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--force` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |
| `--no-daemon` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--note` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

예시:

```text
obit add debug.log 2h
obit add output\shots 1d12h
obit add report.zip --at "2026-12-01 18:00"
```

### `obit approve`
사용법: `obit approve <proposal id>... [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |

### `obit bury`
사용법: `obit bury <path>... [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--force` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |
| `--note` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit cancel`
별칭: `cancel`, `spare`
사용법: `obit cancel <id\|path>... [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |

### `obit completion`
사용법: `obit completion [command]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |

### `obit completion bash`
사용법: `obit completion bash`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--no-descriptions` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit completion fish`
사용법: `obit completion fish [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--no-descriptions` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit completion powershell`
사용법: `obit completion powershell [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--no-descriptions` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit completion zsh`
사용법: `obit completion zsh [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--no-descriptions` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit config`
사용법: `obit config [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--create` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--language` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit config telemetry`
사용법: `obit config telemetry on\|off [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |

### `obit daemon`
사용법: `obit daemon [command]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |

### `obit daemon disable`
사용법: `obit daemon disable [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--dry-run` | 없음 | `false` | 수행할 작업만 표시하고 파일이나 서비스를 변경하지 않습니다. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |

### `obit daemon enable`
사용법: `obit daemon enable [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--dry-run` | 없음 | `false` | 수행할 작업만 표시하고 파일이나 서비스를 변경하지 않습니다. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--resident` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit daemon start`
사용법: `obit daemon start [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |

### `obit daemon status`
사용법: `obit daemon status [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |

### `obit daemon stop`
사용법: `obit daemon stop [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |

### `obit docker`
사용법: `obit docker [command]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |

### `obit docker add`
사용법: `obit docker add <container name\|ID> <lifespan> [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |

### `obit docker list`
사용법: `obit docker list [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |

### `obit doctor`
사용법: `obit doctor [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |
| `--verbose` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit extend`
사용법: `obit extend <id\|path>... <duration> [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |

예시:

```text
obit extend 3 1d
obit extend debug.log 30m
```

### `obit git-clean`
사용법: `obit git-clean [options] [--] [pathspec]... [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |

### `obit graveyard`
별칭: `graveyard`, `graves`
사용법: `obit graveyard [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--here` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |

### `obit help`
사용법: `obit help [command] [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |

### `obit hook`
사용법: `obit hook [command]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |

### `obit hook antigravity`
사용법: `obit hook antigravity [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook audit`
사용법: `obit hook audit [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--lines` | `-n` | `40` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook claude`
사용법: `obit hook claude [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook cline`
사용법: `obit hook cline [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook codebuddy`
사용법: `obit hook codebuddy [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook codebuddy-ide`
사용법: `obit hook codebuddy-ide [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook codex`
사용법: `obit hook codex [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook copilot`
사용법: `obit hook copilot [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook cursor`
사용법: `obit hook cursor [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook droid`
사용법: `obit hook droid [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook gemini`
사용법: `obit hook gemini [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook hermes`
사용법: `obit hook hermes [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook kilocode`
사용법: `obit hook kilocode [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook kimi`
사용법: `obit hook kimi [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook lingma`
사용법: `obit hook lingma [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook minimax-code`
사용법: `obit hook minimax-code [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook omp`
사용법: `obit hook omp [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook openclaw`
사용법: `obit hook openclaw [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook opencode`
사용법: `obit hook opencode [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook pi`
사용법: `obit hook pi [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook qoder`
사용법: `obit hook qoder [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook qoder-cn`
사용법: `obit hook qoder-cn [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook trae`
사용법: `obit hook trae [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook trae-cn`
사용법: `obit hook trae-cn [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook vibe`
사용법: `obit hook vibe [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit hook zcode`
사용법: `obit hook zcode [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--event` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--notice-only` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit init`
사용법: `obit init [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--agent` | 없음 | `[claude]` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--context-menu` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--dry-run` | 없음 | `false` | 수행할 작업만 표시하고 파일이나 서비스를 변경하지 않습니다. |
| `--format` | 없음 | `"text"` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--global` | `-g` | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--guard-rm` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--mcp` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--no-context-menu` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--no-shortcuts` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--no-skill` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--show` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--skill` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--uninstall` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--yes` | 없음 | `false` | 확인하고 실행하며 다시 묻지 않습니다. |

예시:

```text
obit init -g                 # Claude Code, for every project (~/.claude)
obit init                    # Claude Code, this project only (.claude/)
obit init -g --agent codex   # Codex (~/.codex/hooks.json + AGENTS.md)
obit init -g --uninstall     # remove only what Obituary added
obit init -g --show          # what is installed
obit init --show --format markdown > docs/agents.md
```

### `obit list`
별칭: `list`, `ls`
사용법: `obit list [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |

### `obit log`
사용법: `obit log [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |
| `--lines` | `-n` | `20` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit mcp`
사용법: `obit mcp [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |

### `obit mktemp`
사용법: `obit mktemp <lifespan> [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |
| `--name` | 없음 | `"tmp"` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--session` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

예시:

```text
obit mktemp 2h --name screenshots
```

### `obit proposals`
사용법: `obit proposals [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |

### `obit purge`
별칭: `purge`, `cremate`
사용법: `obit purge [id\|original path]... [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--all` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--dry-run` | 없음 | `false` | 수행할 작업만 표시하고 파일이나 서비스를 변경하지 않습니다. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |
| `--yes` | 없음 | `false` | 확인하고 실행하며 다시 묻지 않습니다. |

### `obit reject`
사용법: `obit reject <proposal id>... [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |

### `obit remove-item`
사용법: `obit remove-item [-Recurse] [-Force] [-Path\|-LiteralPath] <path> [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |

### `obit resurrect`
별칭: `resurrect`, `raise`
사용법: `obit resurrect <id\|original path>... \| --last [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |
| `--last` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit rm`
사용법: `obit rm [options] <path>... [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |

### `obit rmdir`
사용법: `obit rmdir [options] <path>... [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |

### `obit rules`
사용법: `obit rules [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |

### `obit rules apply`
사용법: `obit rules apply <path>... [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |
| `--session` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit rules check`
사용법: `obit rules check <path>... [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |
| `--session` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit stats`
별칭: `stats`, `eulogy`
사용법: `obit stats [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |

### `obit sweep`
별칭: `sweep`, `reap`
사용법: `obit sweep [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--dry-run` | 없음 | `false` | 수행할 작업만 표시하고 파일이나 서비스를 변경하지 않습니다. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |

### `obit trust`
사용법: `obit trust [rules file] [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--yes` | 없음 | `false` | 확인하고 실행하며 다시 묻지 않습니다. |

### `obit uninstall`
사용법: `obit uninstall [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--dry-run` | 없음 | `false` | 수행할 작업만 표시하고 파일이나 서비스를 변경하지 않습니다. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--purge-data` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--yes` | 없음 | `false` | 확인하고 실행하며 다시 묻지 않습니다. |

### `obit untrust`
사용법: `obit untrust [rules file] [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |

### `obit update`
사용법: `obit update [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--apply` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--check` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--relaunch-gui` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--wait-pid` | 없음 | `0` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

### `obit watch`
사용법: `obit watch <directory> <lifespan> \| watch --list \| watch --remove <directory> [flags]`

| 매개변수 | 단축 | 기본값 | 설명 |
| --- | --- | --- | --- |
| `--age` | 없음 | `"first-seen"` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--clear-exclusion` | 없음 | `0` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--help` | `-h` | `false` | 도움말을 표시합니다. |
| `--json` | 없음 | `false` | JSON으로 출력합니다. |
| `--list` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--no-daemon` | 없음 | `false` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--policy` | 없음 | `0` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--remove` | 없음 | `""` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |
| `--remove-id` | 없음 | `0` | 이 매개변수의 현재 동작은 소스에서 생성됩니다. 명령줄 help도 확인하세요. |

<!-- generated-command-reference:end -->
