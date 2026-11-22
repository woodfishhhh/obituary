# 빠른 시작

Obituary는 파일과 폴더에 수명을 부여합니다. 수명이 끝나면 묘지에 매장됩니다. 보존 기간(기본 7일) 동안에는 언제든 부활시킬 수 있으며, 그 이후에만 실제로 삭제됩니다. AI 에이전트를 위해 만들어졌으므로 에이전트가 생성한 디버그 스크립트, 로그, 스크린샷, 테스트 출력에는 생성 순간부터 수명이 있습니다. 정리를 기억할 필요가 없고 에이전트의 `rm -rf`가 무엇이든 영구적으로 파괴하지 않습니다.

## 1. 설치

Windows 10 / 11, macOS 및 Linux(x64 및 ARM64):

| | Windows | macOS | Linux |
|---|---|---|---|
| 설치 방법 | Scoop, `install.ps1` 스크립트(winget은 검토 중) | Homebrew, `install.sh` 스크립트 | `install.sh` 스크립트 |
| 만료 확인 | 상주 트레이 프로세스, 작업 스케줄러를 대체 수단으로 사용 | 사용자 launchd 에이전트 | `systemd --user` 타이머, systemd가 없으면 사용자 crontab |
| CLI, 묘지와 부활, 자동 등록(선택 사항), 삭제 가드, 에이전트 연동(hook, Skill, MCP), Docker 컨테이너, 폴더 정책 | 지원 | 지원 | 지원 |
| 트레이, 작은 창, 컨텍스트 메뉴, Explorer의 매장 날짜, 시작 메뉴와 바탕 화면 바로 가기 | 지원 | 대신 CLI 사용: `obit list`, `obit graveyard`, `obit resurrect` | macOS와 동일 |
| 휴지통 모드 | 지원 | 묘지만 사용 | 묘지만 사용 |

**가장 쉬운 방법: 에이전트에게 맡기세요.** 사용하는 에이전트(Claude Code, Codex, Cursor 등)에게 이 문장을 보내세요. Obituary를 설치하고 자신에게 연결한 뒤, 사용자의 결정이 필요한 곳에서 멈춥니다.

```text
Read https://obituary.woodfish.site/agent-install.md and follow it to install Obituary and connect it to yourself.
```

**직접 설치:** 아래에서 운영 체제를 선택한 다음 `obit doctor`를 실행해 설치를 확인하세요.

### macOS / Linux

macOS에서는 Homebrew로 한 줄에 설치할 수 있습니다.

```bash
brew install --cask woodfishhhh/tap/obituary
```

또는 설치 스크립트를 다운로드해 읽은 다음 실행하세요(macOS 및 Linux).

```bash
curl -fsSL https://raw.githubusercontent.com/woodfishhhh/obituary/main/install.sh -o install.sh
bash install.sh
# Custom directory: --install-dir "$HOME/bin"; preview only: --dry-run
```

스크립트는 기본적으로 최신 릴리스를 설치합니다(`--version <version>`으로 다른 버전을 선택할 수 있습니다). 아카이브는 네 부분으로 동시에 다운로드되며(일부 경로는 연결별 속도를 제한합니다), 문제가 생기면 단일 연결로 전환합니다. 운영 체제와 아키텍처를 감지하고 같은 다운로드 원본에서 SHA-256을 확인한 뒤 `obit`와 `obitd`를 `~/.local/bin`에 설치합니다. PATH 안내를 따른 다음 `obit --version`을 실행하세요. 중국 본토 미러를 사용하려면 `--mirror cn`을 지정하세요. 스크립트는 `https://cn.obituary.woodfish.site/install.sh`에 있습니다.

macOS에서 처음 사용할 때는 `obit --version`과 `obitd --help`를 실행하세요. Gatekeeper가 승인을 요청하면 시스템 설정 → 개인 정보 보호 및 보안을 열고, 출처를 확인한 각 실행 파일에서 “그대로 열기”를 선택한 다음 명령을 다시 실행하세요.

WSL에서는 프로젝트와 에이전트 설정을 Linux 파일 시스템(예: `~/projects`)에 두세요. `/mnt/c` 같은 Windows 마운트는 설치에 필요한 안전한 이름 변경을 지원하지 않을 수 있습니다. 이 경우 작업은 오류를 보고하고 원래 설정을 보존합니다. Windows용 Obituary를 직접 사용할 수도 있습니다.

### Windows

::: code-group

```powershell [Scoop]
scoop bucket add obituary https://github.com/woodfishhhh/scoop-bucket
scoop install obituary
```

```powershell [winget]
# In review, available once approved
winget install woodfishhhh.Obituary
```

```powershell [Install script]
# Install script: download it, read it, then run it
# (installs to %LOCALAPPDATA%\Programs\obituary, adds it to your user PATH, creates a Start menu shortcut)
irm https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1 -OutFile install.ps1
.\install.ps1          # installs the latest release; -WhatIf previews, -NoPath leaves PATH alone
```

:::

- 세 가지 방법 모두 GitHub 릴리스에서 같은 아카이브를 다운로드하고, 압축을 풀기 전에 SHA-256을 확인합니다. 불일치하면 설치를 중지합니다.
- 코드 서명: 무료 코드 서명은 [SignPath.io](https://about.signpath.io/)가 제공하고 인증서는 [SignPath Foundation](https://signpath.org/)이 제공합니다. [코드 서명 정책](code-signing)과 [개인정보 보호정책](privacy)을 참조하세요.
- Obituary는 아직 코드 서명이 되지 않았습니다. 브라우저로 아카이브를 다운로드하면 Windows에 “Windows가 PC를 보호했습니다”가 표시될 수 있습니다. “추가 정보 → 실행”을 선택하세요. Scoop, winget, 설치 스크립트에서는 보통 표시되지 않습니다.
- 작은 창 `obit-gui.exe`에는 WebView2 런타임이 필요합니다(Windows 11에 내장되어 있고 대부분의 Windows 10에도 설치되어 있음). x64 전용이며 ARM64 빌드에는 명령줄과 트레이가 있습니다.
설치를 확인하세요.

```powershell
obit doctor              # read-only check: PATH, obitd, runner, Claude Code / Codex integration, data directory, ledger, config (--json)
```

## 2. 에이전트 연결

```powershell
obit init -g                 # Claude Code, all projects; also the Skill (and desktop integration on Windows)
obit init -g --agent codex   # Codex
```

이후 에이전트를 다시 시작하세요. 다른 에이전트, MCP 서버, Agent Skill은 [에이전트 연결](./agents)을 참조하세요.

macOS / Linux에서 Claude Code는 `~/.claude`, Codex는 `~/.codex`, 공유 Skill은 `~/.agents/skills`를 사용합니다. `CLAUDE_CONFIG_DIR`와 `CODEX_HOME` 재정의도 적용됩니다.

데스크톱에서 감독 기능을 켜면 트레이 프로세스가 만료 시점에 처리하고 별도 작업이 한 시간마다 누락된 항목을 확인합니다. 트레이가 없는 버전은 기존처럼 5분마다 확인합니다.

```bash
obit daemon enable --dry-run
obit daemon enable
obit doctor
```

macOS는 로그인한 사용자의 launchd를 사용하고 Linux는 `systemd --user`를 사용합니다. systemd가 없는 Linux에서는 사용자의 crontab에서 5분마다 `obitd --once`를 실행하도록 예약하고 `obitd`, `--home` 데이터 디렉터리, `--config-dir` 설정 디렉터리의 절대 경로를 지정하세요. 에이전트 hook도 작업 기한이 되면 백그라운드 확인을 시작합니다.

## 3. 자주 쓰는 명령

`<id>`를 `obit add` 또는 `obit bury` 영수증의 번호, 혹은 `obit list` / `obit graveyard`에서 표시되는 번호로 바꾸세요.

```powershell
obit add debug.log 2h        # bury it in 2 hours (alias: mark)
obit list                    # what is left and for how long
obit extend <id> 1d          # one more day
obit bury old-build          # bury right away
obit graveyard               # what is in the graveyard and when it will be purged
obit resurrect <id>          # put it back, never overwriting
obit resurrect --last        # undo the last deletion
```

그 밖의 내용은 [명령](./commands)을 참조하세요.

## 다음 단계

| 알아보기 | 읽을 페이지 |
|---|---|
| 등록, 매장, 부활, 영구 삭제가 작동하는 방식과 묘지의 위치 | [작동 방식](./how-it-works) |
| Claude Code, Codex 및 다른 에이전트 연결 | [에이전트 연결](./agents) |
| 등록되는 항목: 에이전트 등록, 컨텍스트 메뉴, 선택적 자동 등록 | [등록 대상](./auto-register) |
| 에이전트의 `rm -rf`를 되돌릴 수 있는 매장으로 바꾸기 | [삭제 가드](./delete-guard) |
| 에이전트가 매장된 파일에 접근할 때 일어나는 일 | [묘지 알림](./grave-notices) |
| Downloads와 Desktop 같은 폴더의 수명 | [폴더 정책](./folder-policies) |
| 명령줄 없이 Obituary 사용 | [창, 트레이 및 컨텍스트 메뉴](./desktop) |
| 세션에서 시작한 Docker 컨테이너 | [Docker 컨테이너](./docker) |
| 모든 명령 | [명령](./commands) |
| Obituary 제거 | [제거](./uninstall) |
| 그 밖의 내용 | [FAQ](./faq) |
