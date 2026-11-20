# 삭제 가드

에이전트가 홈 디렉터리나 전체 드라이브를 지우는 일은 더 이상 가정이 아닙니다. 삭제 가드를 켠 상태에서 에이전트가 `rm -rf build`, `rmdir`, `git clean -fdx`, `Remove-Item -Recurse` 같은 명령을 실행하면 Obituary가 실행을 중지하고 `obit rm -rf build` 같은 동등한 명령을 알려 줍니다. 에이전트가 그 명령을 사용하면 항목은 사라지지 않고 묘지로 이동하며 `obit resurrect <id>`로 되돌릴 수 있습니다.

```powershell
obit init -g --guard-rm                 # Claude Code; add --agent codex for Codex. Off by default
```

## 실제 실행 예

삭제 가드를 켜면 훅이 Claude Code의 Bash 호출 `rm -rf build`을 받아 다음과 같이 응답합니다. `rm`은 실행되지 않습니다.

```json
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Obituary's delete guard is on: `'C:/work/obit.exe' rm -rf build` provides restorable deletion; `obit resurrect` restores graveyard entries."}}
```

에이전트는 Obituary 명령으로 전환하고, 폴더는 묘지로 이동한 뒤 한 명령으로 되돌릴 수 있습니다.

```console
$ obit rm -rf build
$ obit graveyard --here
ID  BURIED            PURGE  SIZE  PATH
#2  2026-11-11 00:36  in 7d  23 B  C:\work\build
$ obit resurrect --last
#2 resurrected  C:\work\build
```

## 원칙

- **거부하고 다시 쓰지 않습니다.** Obituary는 에이전트 명령을 바꾸지 않습니다. Claude Code는 바뀐 명령을 기준으로 권한을 확인하므로 원래 명령에 대한 `deny` 규칙이 무력화될 수 있습니다. 원래 실행을 거부하고 에이전트가 동등한 명령을 새로 내리게 하면 권한 규칙이 그대로 적용됩니다.
- **문자 그대로의 삭제는 중지합니다.** `rm`、`rmdir`、`git clean`、`Remove-Item`（`del`、`rd`、`ri` 같은 별칭 포함）가 `&&`、`||`、`;` 뒤, 파이프라인, `$(…)`, 서브셸, 조건문, 반복문 등 어떤 명령 위치에 나타나도 실행을 중지합니다. 예를 들어 `rm -rf build && npm run build`, `rm -rf build 2>/dev/null`, `git clean -fdx && npm ci`입니다. 문자열, 주석 또는 `echo` 인자 속 `rm`은 제외하며 `rm --help`, `git clean -n`, `Remove-Item -WhatIf` 같은 미리 보기 역시 제외합니다.
- **실행 중 선택되는 대상은 먼저 나열합니다.** Bash의 `find -delete`, `find -exec rm`（`-execdir` 포함）, `xargs rm`, PowerShell의 `Get-ChildItem … | Remove-Item`는 다시 쓰지 않고 거부합니다. 먼저 일치 항목을 검토하고 명시적 경로를 `obit rm -- <path>...`에 하나씩 또는 묶어서 전달하세요. 검색이나 경로 목록만 만드는 `find`는 실행됩니다. `sudo rm`이나 `bash -c '…'` 같은 임의의 래퍼는 검사하지 않습니다.
- **실제 삭제로 돌아가지 않습니다.** `obit rm`、`obit remove-item`、`obit git-clean`에 전달한 모든 경로는 매장됩니다. 드라이브 루트나 시스템 폴더, 사용자 프로필 등 거부되는 대상은 `rm` 형식으로 오류를 반환하고 종료 코드는 0이 아닙니다.
- **명령이 중간에 바뀌는 일을 고려합니다.** 셸의 인용과 이스케이프, 명령을 다시 쓰는 프록시 때문에 에이전트가 의도한 명령과 실제 명령이 달라질 수 있습니다. 한 글자만 사라져도 `rm -rf "$OUT"/`가 `rm -rf /`가 됩니다. 그래서 `obit rm`는 실행 전에 디스크의 실제 경로를 확인합니다.
- **Windows Codex에서는 따옴표 안의 치환도 검사합니다.** 다른 셸을 거치므로 Bash에서는 문자로 남을 내용이 실행될 수 있습니다. Windows Codex의 명령에서는 `$(…)`를 Bash와 PowerShell 양쪽으로 읽고, 따옴표와 인용 heredoc, 주석 안에서도 삭제를 검사합니다. 삭제가 있으면 중지합니다. 백틱이 있는 텍스트는 편집 도구로 파일에 쓰고 경로를 명령에 전달하면 그대로 사용할 수 있습니다.
- Git이 추적하는 파일도 매장되므로 커밋하지 않은 변경을 복구할 수 있습니다. 이는 에이전트가 의도적으로 삭제한 경우이며 추적 파일을 건드리지 않는 자동 등록과는 다릅니다.
- `.git`이 있는 폴더의 `obit rm -rf`는 전체 저장소를 매장합니다. 실제 `rm -rf`라면 파괴하지만 매장하면 되돌릴 수 있습니다. 강제할 수 없는 시스템 폴더와 사용자 폴더 보호는 계속 적용됩니다.
- **동등한 명령의 범위는 의도적으로 좁습니다.** 원래 명령보다 더 많은 대상을 선택하면 안 되므로 셸 간 변환을 하지 않고 아래 표의 형식에만 동등한 명령을 제시합니다. 그 밖에는 실행을 중지하고, 이유와 함께 `obit bury <path>`（문자 그대로의 경로）와 `obit git-clean`이 복구 가능한 삭제를 제공한다고 설명합니다.

## 무엇이 멈추고 무엇을 통과하는가

| 호스트 | 동등한 명령과 함께 중지 | 이유와 일반 안내와 함께 중지 | 통과 |
|---|---|---|---|
| Claude Code（Bash 도구） | `rm`、`rmdir`、`git clean`, 필요하면 문자 그대로의 `cd x`를 앞에 둘 수 있음（`&&`、`;` 또는 줄바꿈으로 연결）；`rm -f *.log`（폴더에 `-`로 시작하는 파일이 없을 때）, `rm -rf ../dist`, `rm -- "$target"`（변수는 `--` 뒤） | 그 밖의 문자 그대로의 삭제：앞뒤에 다른 문장（`npm …`、`trap`、`hash`）, `--` 앞의 변수, `/`로 시작하는 POSIX 절대 경로, 리디렉션, `\|\|명령어를 실행하면, `, 파이프라인, `$(…)`, 대화형 삭제, 경로가 있는 `/usr/bin/rm` | 삭제하지 않는 명령 |
| WSL의 `/mnt/<drive>` 아래 Codex | `..`, `~`, 와일드카드, 따옴표가 없는 상대 경로를 인자로 받는 단일 `rm` / `rmdir` | `git clean`과 WSL 자체 파일 시스템 경로를 포함한 그 밖의 문자 그대로의 삭제 | WSL 자체 파일 시스템 안의 작업 디렉터리 |
| Windows PowerShell로 선언된 Codex | 하나의 문자 그대로의 경로만 받는 단일 `Remove-Item` 또는 별칭（`-Recurse`, `-Force` 허용） | 여러 문장, 파이프라인, `-ErrorAction` 같은 다른 인자, provider 경로, `~`, 와일드카드, 백틱 등 그 밖의 문자 그대로의 삭제 | 따옴표 안의 텍스트 |
| Windows지만 셸을 알 수 없는 Codex | — | 원시 백틱 또는 `$(…)` 조각 안의 삭제 | 그 밖의 명령 |
| Linux의 pwsh, `git rm` | — | — | 모두 통과 |

동등한 명령을 제공하려면 삭제 명령이 전체 명령의 **마지막 부분**이어야 하며 앞부분은 `&&`, `;` 또는 줄바꿈으로만 연결되어야 합니다. 마침표나 공백으로 끝나는 파일 이름에는 Windows가 다른 이름으로 정규화하므로 동등한 명령을 제공하지 않습니다. 이 경우에도 문자 그대로의 삭제는 중지합니다.

## 직접 사용

`obit rm`, `obit rmdir`, `obit git-clean`은 `rm`, `rmdir`, `git clean`의 일반적인 비대화형 옵션을 받습니다. `-i`나 `rmdir -p` 같은 미지원 옵션은 조용히 무시하지 않고 오류로 처리합니다. `obit remove-item`은 문자 그대로의 경로 하나만 받는 `Remove-Item` 간단한 명령입니다. 와일드카드는 Bash에서 `obit rm *.log`를 사용하세요.

`OBIT_GUARD_RM=0`으로 잠시 끌 수 있습니다. `obit stats`에서 중지한 삭제 횟수와 매장한 용량을 확인할 수 있습니다.
