# 에이전트 연결

## Claude Code

```powershell
obit init -g                         # all projects (~/.claude); obit init alone installs into this project (.claude/)
obit init -g --show --agent claude   # what is installed
obit init -g --uninstall             # remove only what Obituary added
```

`obit init`은 `settings.json`에 `obit hook claude`를 등록합니다. 실행 시점은 SessionStart, 각 도구 호출 전, 파일 쓰기 후, Stop, SessionEnd입니다. 에이전트용 `OBIT.md`를 작성하고 `CLAUDE.md`에 참조 한 줄을 추가합니다. 사용자 단위(`-g`)에서는 `~/.claude/CLAUDE.md`에 `@OBIT.md`를, 프로젝트 단위에서는 프로젝트의 `CLAUDE.md`에 `@.claude/OBIT.md`를 추가합니다.

- 설정과 키 순서는 그대로입니다. 다시 실행해도 중복 항목은 생기지 않으며, `obit.exe`를 옮긴 뒤 다시 실행하면 경로만 갱신됩니다.
- 수정 전에는 항상 백업합니다. 처음에는 `.bak`, 이후에는 `.bak.1`, `.bak.2` 순으로 저장하며 이전 백업은 덮어쓰지 않습니다.
- 훅에서 오류가 발생해도 에이전트는 중단 없이 계속 실행됩니다. 오류는 `hook.log`에 기록됩니다. `OBIT_DISABLED=1`로 훅을 일시적으로 끌 수 있습니다.

## Codex

```powershell
obit init -g --agent codex   # ~/.codex/hooks.json plus a block in AGENTS.md; obit init --agent codex installs into this project
```

- **Codex는 사용자가 신뢰하도록 설정한 훅만 실행합니다.** 설치 후 Codex에서 `/hooks`를 열고 Obituary 훅을 신뢰하도록 설정하세요. 프로젝트 훅은 프로젝트 자체도 신뢰 대상으로 지정해야 합니다.
- Windows와 WSL은 같은 `hooks.json`을 사용합니다. WSL에서 받은 `/mnt/<drive>/…` 경로는 `<drive>:\…` 형식으로 변환합니다. WSL 자체 파일 시스템에 있는 경로(`/home/…`)는 현재 건너뜁니다.
- Codex는 `apply_patch`로 파일을 편집합니다. Obituary는 패치에서 추가, 수정, 이동한 파일만 확인하며 삭제한 파일은 세지 않습니다.

## 기타 에이전트

```powershell
obit init -g --show                         # every supported agent, how far each integration goes, what is installed
obit init -g --agent gemini,cursor          # install one or several; without -g, into this project only
obit init -g --agent gemini --mcp --skill   # also register the MCP server and install the Skill (where supported)
obit init -g --agent gemini --uninstall     # remove only what Obituary added
```

`--agent`를 생략하면 Claude Code만 설정합니다. Obituary가 임의로 다른 에이전트에 설치하지 않습니다.

| 방법 | 에이전트 |
|---|---|
| 네이티브 훅 | GitHub Copilot(VS Code 및 Copilot CLI), Cursor, Gemini CLI, Factory Droid, Trae / Trae CN, Kimi, Mistral Vibe, Google Antigravity, Tencent CodeBuddy(CLI 및 IDE), Qoder / Qoder CN / Lingma, Zhipu ZCode |
| 플러그인(Obituary가 `obit hook`을 호출하는 작은 플러그인을 설치) | OpenCode, OpenClaw, Pi, Oh My Pi, Hermes, Cline, Kilo Code, MiniMax Code, DeepSeek Harness |
| 규칙 파일 및 MCP만 | Roo Code, Windsurf, WorkBuddy, QoderWork |

::: tip 에이전트별 지원 기능
쓰기 후 등록, 세션 시작 알림, 삭제 가드, MCP, Skill의 지원 여부는 에이전트마다 다릅니다. 저장소의 [에이전트별 지원 표](https://github.com/woodfishhhh/obituary/blob/main/docs/agents.md)에서 각 기능의 지원 여부(지원, 제한적 지원, 미지원)와 이유를 확인하세요.
:::

공개 문서만으로는 설정을 안전하게 작성하기 어려운 경우 `obit init`은 전체 수동 절차를 출력하고 수동 작업이 필요한 미완료 상태로 보고합니다. 설치하지 않은 항목을 설치했다고 표시하지 않습니다.

## MCP 서버와 Agent Skill

```powershell
obit init -g --mcp --skill                 # Claude Code; add --agent codex for Codex. Neither is installed by default
```

- **`obit mcp`**는 표준 입출력(stdio)으로 동작하는 MCP 서버입니다. 에이전트는 `obit_mark`, `obit_mktemp`, `obit_bury`, `obit_resurrect`, `obit_extend`, `obit_spare`과 조회용 도구 `obit_list`, `obit_graveyard`, `obit_proposals`, `obit_stats`을 호출할 수 있습니다.
- **사용자에게 맡긴 결정은 에이전트가 대신할 수 없습니다.** 제안 승인·거부, `.obitrules` 신뢰, 묘지 항목 영구 삭제, 설정 변경에 해당하는 MCP 도구는 없습니다. 에이전트는 제안을 전달할 수만 있습니다.
- **Agent Skill**은 결과물에 언제 수명을 지정할지, 부활할 수 있게 삭제하는 방법, 어떤 결정을 사용자에게 맡길지 설명하는 `SKILL.md` 파일입니다. 이 파일을 직접 수정하면 Obituary는 더 이상 덮어쓰거나 제거하지 않습니다.
- 설치한 뒤 에이전트를 다시 시작하세요. `obit doctor`으로 두 기능이 설치되었고 현재의 `obit.exe`를 가리키는지 확인할 수 있습니다.
