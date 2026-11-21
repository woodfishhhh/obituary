# 설정 참고

Obituary는 설정 디렉터리의 `config.toml`을 읽습니다. 모든 키는 생략할 수 있으며 생략하면 아래 기본값을 사용합니다. `obit config`에서 파일 위치와 적용된 값을 확인할 수 있습니다.

## 키

| 키 | 기본값 | 값 또는 형식 | 동작 |
| --- | --- | --- | --- |
| `codex_windows_shell` | 설정 안 함 | `bash`, `powershell` | Windows에서 Codex가 사용하는 shell을 선언합니다. 설정하지 않으면 모호한 삭제 명령을 그대로 통과시킵니다. |
| `agent_shells.<agent>` | 설정 안 함 | `bash`, `powershell` | hook agent 하나의 Windows shell을 선언합니다. |
| `graveyard.mode` | `graveyard` | `graveyard`, `recycle` | 만료 항목을 묘지 또는 시스템 휴지통으로 이동합니다. |
| `graveyard.retention_days` | `7` | `1`~`3650` 정수 | 묘지 항목을 복원할 수 있는 일수입니다. |
| `watch.stable_window` | `10m` | `30s`, `10m`, `1h` 같은 Go duration | 폴더 정책 이동 전에 안정화될 때까지 기다리는 시간입니다. 양수여야 합니다. |
| `explorer.enabled` | `true` | `true`, `false` | Explorer 날짜 레이블을 기록합니다. |
| `explorer.everywhere` | `false` | `true`, `false` | 지원되는 모든 폴더에 날짜를 표시합니다. |
| `explorer.in_git` | `false` | `true`, `false` | Git 작업 트리 안에도 날짜를 표시합니다. |
| `ui.language` | `auto` | `auto`, `en`, `zh-CN` | CLI, 창, 트레이의 언어입니다. |
| `ui.tray` | `true` | `true`, `false` | 트레이 또는 메뉴 막대 진입점을 사용합니다. |
| `ui.taskbar` | `true` | `true`, `false` | Windows 창을 별도 작업 표시줄 항목으로 표시합니다. |
| `hooks.session_summary` | `false` | `true`, `false` | 세션 시작 시 요약을 표시합니다. |
| `hooks.session_cleanup` | `true` | `true`, `false` | 이 세션에서 새로 만든 미분류 파일을 정리합니다. |
| `proposals.mode` | `off` | `off`, `tiered`, `auto`, `ask` | 자동 등록을 승인 대기할 범위를 정합니다. |
| `update.check` | `true` | `true`, `false` | 안정 버전 업데이트를 자동 확인합니다. |
| `telemetry.enabled` | `true` | `true`, `false` | 사용 통계 전송을 허용합니다. 업데이트 확인과 별개입니다. |

`agent_shells`는 agent 이름을 키로 하는 표입니다. 예: `[agent_shells] codex = "powershell"`. `codex_windows_shell`은 이전 Codex 표기이며 두 값이 다르면 오류입니다.

## 형식과 오류

duration은 Go 형식을 사용합니다. `ns`, `us`, `ms`, `s`, `m`, `h`를 조합해 `1h30m`처럼 쓸 수 있습니다. 보존 일수는 `1`~`3650`, `graveyard.mode`, `ui.language`, `proposals.mode`는 표에 있는 값만 허용합니다.

알 수 없는 키는 무시되지 않습니다. 설정 경로, 줄 번호, 전체 키 이름이 오류에 표시되며 가까운 철자에는 제안도 표시됩니다. TOML 문법 오류도 `obit config`를 실패시킵니다.

## 자주 쓰는 명령

```powershell
obit config
obit config --create
obit config --language en
obit config telemetry off
obit config telemetry on
```

`--create`는 파일이 없을 때만 주석이 있는 기본 템플릿을 씁니다. `--language`의 허용 값은 `auto`, `en`, `zh-CN`입니다. `telemetry on|off`는 `telemetry.enabled`만 변경하며 `[update] check`에는 영향을 주지 않습니다.
