# 제거

```powershell
obit uninstall --dry-run      # see what would be removed
obit uninstall                # remove everything Obituary set up, after you confirm
.\install.ps1 -Uninstall      # if you used the install script: also remove the program files, PATH entry and Start menu shortcut
```

`obit uninstall`은 Obituary가 추가한 항목만 제거합니다. 사용자 수준과 현재 Git 프로젝트에 설치한 에이전트 훅, MCP, Skill을 비롯해 로그인 시 자동 시작 설정, 예약된 작업, 실행 중인 데몬, 상황에 맞는 메뉴, 파일 탐색기의 날짜 표시, 폴더 정책이 대상입니다.

## 묘지와 장부는 기본적으로 남습니다

기본적으로 묘지와 장부를 보존하고 묘지의 위치와 보관량을 알려 줍니다. 둘 다 지우려면 `--purge-data`를 추가하세요. 먼저 묘지 내용을 보여 준 뒤 다시 확인을 요청합니다.

## 다른 프로젝트의 훅

다른 프로젝트에도 프로젝트 단위로 훅을 설치했다면 프로그램을 제거하기 전에 각 프로젝트에서 `obit uninstall`을 실행하세요. 그렇지 않으면 `obit`이 없어져 해당 훅이 작동하지 않습니다.

## Scoop과 winget

먼저 `obit uninstall`을 실행한 뒤 설치에 사용한 패키지 관리자의 제거 명령을 실행하세요.
