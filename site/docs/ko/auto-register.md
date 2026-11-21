# 등록 대상

## 등록할 항목은 사용자와 에이전트가 정하고 매장은 정해진 시간에 합니다

기본 설정(`[proposals] mode = "off"`)에서는 사용자와 에이전트가 등록할 대상을 정합니다.

- **에이전트는 자신의 결과물을 등록합니다.** 작성한 디버그 스크립트, 로그, 스크린샷에 `obit mark <path> 2d`로 수명을 지정합니다. 임시 작업 공간은 `obit mktemp 1d`로 만들고 보관 여부가 불확실한 항목은 `obit bury`로 매장합니다. `obit init`가 작성하는 에이전트용 안내에도 이 내용이 들어 있습니다.
- **사용자는 상황에 맞는 메뉴에서 추가합니다.** Windows 파일 탐색기에서 파일이나 폴더를 오른쪽 클릭하고(여러 개를 선택해도 됩니다) ‘Add to Obituary’에서 1일, 7일, 30일 또는 즉시 매장(복구 가능)을 선택하세요. [창과 트레이, 상황에 맞는 메뉴](./desktop)를 참고하세요.
- **폴더 정책**으로 다운로드 같은 폴더에 수명을 지정할 수 있습니다. [폴더 정책](./folder-policies)을 참고하세요.

등록된 항목은 만료되면 매장되며, 7일 동안 묘지에서 원래 상태로 부활시킬 수 있습니다. 그 밖의 항목은 저절로 들어오지 않습니다.

- **세션 마무리.** Claude Code나 Codex 에이전트가 작업을 마치려 하면, Obituary가 그 세션에서 에이전트가 만들었지만 분류하지 않은 파일을 나열합니다. 디버그 스크립트, 로그, 스크린샷에는 수명을 지정하거나 매장하고 최종 결과물은 남기도록 안내합니다. 세션당 최대 한 번만 알리며 대상이 없으면 알리지 않습니다. 다른 에이전트는 다음 세션 시작 때 알림을 받습니다. `[hooks] session_cleanup = false`로 끌 수 있습니다.
- **이전 버전에서 업그레이드하면** 자동 등록이 한 번 꺼집니다. 이후에 사용자가 변경한 설정은 그대로 유지됩니다. 이전에 자동 등록한 의존성, 빌드 결과물, 캐시는 등록을 해제하여 수명 없이 제자리에 둡니다.

## 원할 때 자동 등록 켜기

창의 Settings나 For you에서 바꾸거나 `config.toml`에서 `[proposals] mode`를 설정하세요.

- `"tiered"`: 세션 중 에이전트가 만든 파일과 해당 세션에서 프로젝트용으로 시작한 [Docker 컨테이너](./docker)를 바로 등록합니다. 파일은 훅에서 에이전트의 쓰기를 확인했고, 세션 시작 시 존재하지 않았으며, 파일 시스템의 생성 시각도 세션 시작 이후여야 합니다. 등록 후 에이전트에 알리고 `obit proposals`와 For you의 ‘Handled automatically’에 표시합니다. 시작 전부터 있던 파일이나 훅이 쓰기를 관찰하지 못한 파일은 제안만 합니다. `obit approve <id>`는 제안의 항목을 등록하고 이후 같은 프로젝트에서 규칙에 맞는 항목을 바로 등록하도록 합니다. `obit reject <id>`는 이 프로젝트의 항목을 더 이상 자동 등록하지 않겠다는 뜻입니다.
- `"auto"`: 자동 등록에서 찾은 항목은 바로 등록하고 나중에 알립니다.
- `"ask"`: 먼저 모든 항목을 제안하고 승인을 기다립니다.

```powershell
obit proposals           # proposals waiting for you, and what was registered without asking
obit approve 3           # approve: register what it lists
obit reject 3            # reject
obit spare 12            # undo an automatic registration: it stays where it is, without a lifespan
obit extend 12 1w        # keep it longer
```

- **의존성, 빌드 결과물, 캐시**(`node_modules`, `dist`, `.turbo`, `__pycache__` 등)는 어떤 모드에서도 자동 등록하거나 제안하지 않습니다. 정리하려면 직접 `obit mark`로 등록하세요.
- **큰 항목**(1GB 초과 또는 파일 200개 초과)을 에이전트가 만들지 않았다면 이 역시 제안부터 합니다.
- **세션 전후 비교.** 세션 시작 시 스냅샷을 데이터 디렉터리에 저장해 종료 시점과 비교합니다. 저장소에는 아무것도 추가하지 않습니다. 규칙에 맞는 항목은 모드에 따라 처리하고 어떤 규칙과도 일치하지 않는 새 파일은 등록도 제안도 하지 않습니다.
- **모드를 바꾸면** 백그라운드 프로그램이 대기 중인 제안을 새 모드에 따라 다시 분류합니다.
- **매장 전 Git에 다시 확인합니다.** 규칙이나 제안으로 등록한 항목이 만료 시점에 커밋되어 있다면 매장하지 않습니다.
- **제안은 자동으로 만료됩니다.** 커밋되거나 삭제된 항목은 제외하고 비게 된 제안은 철회합니다. 이 사실은 감사 로그에 남지만 사용자의 거부로 집계하지 않습니다.

## 규칙

```powershell
obit rules                       # the rules in effect here (built-in plus a trusted .obitrules)
obit rules check debug.log out/  # preview what would be registered and by which rule (changes nothing)
obit rules apply debug.log       # register by rule
```

`obit rules apply`로 언제든 규칙에 따라 등록할 수 있습니다. 자동 등록을 켜면 훅도 같은 규칙을 따릅니다. 기본 규칙은 `.playwright-mcp/`, `test-results/`, `*.log` 등 에이전트가 흔히 남기는 작업 파일에 적용됩니다. 편집 후 남은 백업(`*.bak`, `*.bak.*`, `*.orig`, `*.rej`, `*.old`, `*.backup`, `*~`)은 7일간 보관하고 LevelDB의 `LOG.old`는 건드리지 않습니다. 프로젝트 루트의 `.obitrules`는 `.gitignore` 문법을 사용하며 각 패턴 뒤에 수명을 적습니다.

```
output/**        7d      # every item under output gets its own lifespan
!output/keep/**          # never touch (negation wins, even over registering a parent folder)
test-results/    off     # turn off a built-in rule
```

`.gitignore`와 달리 마지막으로 일치한 규칙보다 제외 규칙이 우선합니다. Windows에서는 대소문자를 구분하지 않습니다. 닫지 않은 `[` 같은 잘못된 패턴이 있으면 파일 이름과 줄 번호를 표시하고 해당 프로젝트의 자동 등록을 일시 중지합니다.

기간 단위는 `s`, `m`, `h`, `d`, `w`이며 `1d12h`처럼 조합할 수 있습니다.

## 자동 등록에 대한 엄격한 규칙

- Git 작업 트리 안에서 Git이 추적하지 않는 항목만 등록합니다.
- Git이 추적하는 파일을 하나라도 포함한 폴더는 건드리지 않습니다.
- Git이 응답하지 못하면 건너뜁니다.
- 기존의 수명은 결코 단축되지 않습니다.
- 사용자가 거부한 프로젝트에서는 어느 모드에서도 직접 등록하지 않습니다. 폴더 정책 적용과 `.obitrules` 신뢰 여부는 항상 사용자 결정을 기다립니다.

## 프로젝트 규칙은 먼저 신뢰해야 합니다

저장소의 `.obitrules`는 내용을 검토하고 `obit trust`를 실행하기 전까지 효력이 없습니다. 다른 사람의 저장소가 규칙을 이용해 개인 파일을 옮기는 일을 막기 위해서입니다. 신뢰할 때 파일의 SHA-256을 기록하고 내용이 바뀌면 다시 검토해야 합니다. 비대화형 세션에서는 `--yes`가 필요하고 CI에서는 `OBIT_TRUST_PROJECT_RULES=1`을 설정할 수 있습니다.

## 만료되면 자동으로 매장되는 임시 폴더

```powershell
$dir = obit mktemp 1d            # .obit/tmp/ in a project (not in Git), otherwise the system temp folder
```

Bash에서는 `dir=$(obit mktemp 1d)`을 사용합니다.
