# 묘지 알림

먼저 매장해 두더라도 나중에 찾을 수 있어야 부활시킬 수 있습니다. 에이전트의 도구가 이미 매장됐지만 아직 부활시킬 수 있는 경로나, 매장된 디렉터리 안의 항목에 접근하면 Obituary가 다음 정보를 알려 줍니다.

- 무덤 번호
- 매장한 시각
- 부활시킬 수 있는 남은 시간
- 부활 명령 `obit resurrect <id>`

## 알림만 보냅니다

- 도구 호출을 차단하거나 변경하지 않습니다.
- 같은 무덤은 세션마다 한 번만 알립니다.
- Claude Code와 Codex는 도구를 실행하기 전에 알림을 받습니다. 다른 에이전트의 알림 시점은 `obit init --show`에서 확인할 수 있습니다.
- 그 외에는 작업을 방해하지 않습니다. 에이전트는 Obituary를 사용할 수 있다는 것만 알고 작업을 시작하며, 먼저 목록을 읽을 필요가 없습니다.

이미 설치했다면 `obit init`을 다시 실행해 묘지 알림을 적용하세요.

## 세션 요약 (선택 사항)

토큰을 아끼기 위해 세션을 시작할 때는 사용자의 결정이 필요한 제안과 방금 자동 등록한 항목만 알려 줍니다. 요약도 보려면 작은 창의 설정에서 ‘Session summary’를 켜거나 다음과 같이 설정하세요.

```toml
[hooks]
session_summary = true
```

그러면 이 프로젝트에서 24시간 안에 만료되는 항목과 최근 매장한 항목 중 아직 부활시킬 수 있는 항목도 알려 줍니다. `obit mktemp`로 만든 임시 디렉터리는 제외합니다.

## 직접 확인하기

```powershell
obit graveyard           # what is in the graveyard and when it will be purged
obit graveyard --here    # this folder only
obit resurrect <id>      # put it back; if the name is taken, it comes back renamed, never overwriting
obit resurrect --last    # undo the last deletion: everything one obit bury / obit rm buried comes back
```
