# 문제 해결

먼저 `obit doctor`를 실행하세요. 설치, 통합, 백그라운드 서비스와 데이터를 변경하지 않고 검사합니다. 자세한 결과에는 `--json` 또는 `--verbose`를 사용합니다.

## PATH와 체크섬

**증상:** `obit`을 찾지 못하거나 아카이브 체크섬이 맞지 않습니다.

**해결:** 같은 릴리스의 `checksums.txt`를 다시 받아 다운로드 폴더에서 `sha256sum --check --strict checksums.txt`를 실행하세요. Windows PowerShell에서는 `Get-FileHash .\obit-*.zip -Algorithm SHA256` 결과를 비교합니다. 불일치 파일은 실행하지 말고 PATH를 바꾼 뒤 새 터미널을 여세요.

## 데스크톱

Windows SmartScreen의 서명되지 않은 프로그램 경고는 SHA-256을 확인한 뒤 “추가 정보 → 실행”을 선택합니다. WebView2가 없으면 Microsoft Edge WebView2 Runtime을 설치하세요. Windows 창은 현재 x64만 제공합니다.

macOS 앱은 Developer ID 서명이나 공증이 없습니다. 체크섬을 확인한 뒤 “시스템 설정 → 개인정보 보호 및 보안 → 확인 없이 열기”에서 첫 실행을 허용합니다. Gatekeeper를 끄거나 손상·악성코드 경고 앱을 허용하지 마세요. 메뉴 막대가 로그인 때 시작되지 않으면 `obit daemon status`와 `launchctl list | grep obituary`를 확인합니다.

Ubuntu 24.04 창에는 GTK 3과 WebKitGTK 4.1이 필요합니다.

```bash
sudo apt update
sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0
obit doctor
```

`systemctl --user status obituary.timer`, `systemctl --user list-timers | grep obituary`, `crontab -l`로 스케줄러를 확인합니다.

## 백그라운드, 업데이트와 파일

`obit daemon status`로 상태를 확인하고 즉시 처리하려면 `obit sweep`, 스케줄러를 다시 켜려면 `obit daemon enable`을 사용합니다. 중국 미러 설치는 `bash install.sh --mirror cn`입니다. 서로 다른 릴리스의 아카이브와 체크섬을 섞지 마세요.

WSL 탑재 드라이브 이름을 바꾼 뒤에는 실제 Windows 경로를 확인하세요. 사용 중인 파일은 원래 위치에 남고 재시도됩니다. TOML 오류는 `obit config`가 파일과 줄을 알려 줍니다.

## 통합과 컨테이너

hook이 작동하지 않으면 `obit init --show`, `obit init --dry-run`, `obit doctor --verbose`를 실행하고 `OBIT_DISABLED=1`이 없는지 확인하세요. 감사 정보는 `OBIT_HOOK_AUDIT=1 obit hook audit`로 확인합니다. Docker를 시작한 뒤 컨테이너 제안을 승인하세요. 복원 대상이 이미 있으면 `obit resurrect`는 덮어쓰지 않고 충돌 없는 이름으로 복원합니다.
