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
