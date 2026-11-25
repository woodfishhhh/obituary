# 폴더 정책

폴더 정책을 설정하면 그 폴더에 들어오는 항목마다 수명이 따로 생깁니다. 다운로드 폴더가 대표적인 예입니다.

```powershell
obit watch C:\Users\you\Downloads 2w        # files that land in Downloads are buried two weeks later
obit watch --list                           # folder policies, their exemptions and quiet periods
obit watch --remove C:\Users\you\Downloads  # remove a policy and the lifespans it registered
```

macOS와 Linux에서도 정책을 승인하면 폴더 바로 아래의 항목마다 별도로 수명이 정해지며, 기본적으로 처음 발견한 시점부터 계산합니다. 정책 폴더 자체는 보호되고 만료된 하위 폴더는 제안만 생성합니다. 다운로드 임시 파일은 건너뜁니다.

일반 파일은 만료된 뒤 기본적으로 10분 더 기다립니다. 크기나 수정 시간이 바뀌거나, 파일이 열려 있거나, 프로세스 정보를 읽을 수 없으면 파일을 그대로 두고 시간을 다시 잽니다. Windows에서는 이동하는 동안 쓰기를 차단합니다. macOS와 Linux에서는 확인 후 이름을 바꾸기 전에 다른 프로세스가 파일을 열 수 있습니다.

개별 승인이 필요하면 `[watch]`에 `file_expiry = "propose"`를 설정하고 `obit proposals`로 검토한 뒤 `obit approve <ID>`로 승인하세요. 만료된 하위 폴더는 계속 제안만 생성하며 macOS와 Linux에서는 승인해도 그대로 남습니다.

```sh
obit watch "$HOME/Downloads" 2w
```

## 적용 조건

- **정책은 승인을 받아야 합니다.** `obit watch`는 제안을 만듭니다. `obit approve`를 실행하거나 작은 창의 ‘For you’ 페이지에서 승인하면 정책이 적용됩니다.
- **10분 동안 변경되지 않은 파일만 매장합니다.** 수명이 만료되고 최근 10분 동안 변경된 적이 없어야 합니다.
- **하위 폴더는 따로 승인합니다.** 하위 폴더가 만료되면 제안만 만들고, 사용자가 승인한 뒤에 매장합니다.
- 정책을 제거하면 그 정책이 등록한 수명도 취소됩니다.

## 파일 탐색기의 날짜 표시 {#dates-in-explorer}

다운로드 폴더, 바탕 화면, 정책을 적용한 폴더에서 추적 중인 항목은 파일 탐색기에 ‘name (11-13 burial)’처럼 표시됩니다. 실제 파일 이름은 바뀌지 않습니다. 날짜 표시는 Obituary의 표시 언어를 따릅니다. 중국어 간체에서는 ‘name（11-13 下葬）’로, 그 외에는 영어로 표시됩니다.

- 날짜 정보는 폴더의 숨김 파일 `desktop.ini`에서 가져옵니다.
- `config.toml`의 `[explorer]` 섹션에서 `enabled = false`로 날짜 표시를 끄거나 `everywhere = true`로 모든 위치에서 표시할 수 있습니다.
- 파일 탐색기는 폴더 보기를 캐시하므로 수명을 늘린 뒤 같은 창에 최대 1분 동안 이전 날짜가 남을 수 있습니다.

Finder와 Dolphin은 파일 이름을 바꾸지 않고 매장 날짜를 태그로 표시합니다. 수명을 연장하면 날짜도 바뀌며, 추적을 해제하거나 프로그램을 제거하면 Obituary가 추가한 태그만 지웁니다. Nautilus에서는 `obit list` 명령으로 확인하세요.
