# 개인정보 처리방침

Obituary의 장부, 묘지, 로그, 설정은 사용자 컴퓨터에 보관됩니다. 충돌 보고서, 파일, 경로 또는 파일 내용을 업로드하지 않습니다.

## 사용 통계

사용 통계는 기본적으로 켜져 있으며, 사용 중인 설치 수와 버전별 분포를 파악하는 데만 쓰입니다. 실제 사용 중에 Obituary는 무작위 설치 ID, 버전, 운영체제, 아키텍처, 인터페이스 언어, 설치 채널을 24시간에 최대 한 번 전송할 수 있습니다. 파일, 경로, 명령은 보내지 않으며 IP 주소나 User-Agent 헤더도 저장하지 않습니다. `obit config telemetry off`를 실행하거나 `OBIT_TELEMETRY=0`을 설정하면 전송을 끌 수 있습니다. CI 환경이거나 `DO_NOT_TRACK=1`이 설정되어 있으면 전송하지 않습니다.

서버는 IP 주소, User-Agent, 원본 설치 ID를 보관하지 않습니다. 별도 키로 계산한 해시값은 같은 설치에서 온 보고서를 연결할 수 있지만 개인을 식별하지는 않습니다. 일별 활동 기록은 400일 후 만료됩니다. 중복 집계를 막기 위해 해시값과 최초 및 마지막 관측 날짜만 장기 보관합니다. 성공적으로 전송된 보고서는 설치 채널별 총건수만 집계합니다. 유지관리자만 인증을 거쳐 대시보드를 볼 수 있습니다.

보고를 끄면 로컬 설치 ID와 전송 대기 중인 보고서가 지워집니다. 업데이트 확인은 별도로 설정합니다.

## 새 버전 확인

- 하루에 최대 한 번 `https://api.github.com`에서 Obituary 저장소의 최신 릴리스에 관한 공개 정보를 읽습니다. GitHub가 응답하지 않으면 중국 본토 미러의 `https://cn.obituary.woodfish.site/download/latest.json`을 읽습니다.
- 요청에 포함되는 정보는 일반 웹 요청과 같습니다. 사용자의 IP 주소와 Obituary 버전이 표시된 User-Agent(예: `Obituary/1.0.0`)뿐입니다.
- 언제든지 창의 설정에서 ‘Check for updates automatically’를 끄거나 `%APPDATA%\obituary\config.toml`에 다음 내용을 추가해 자동 확인을 중지할 수 있습니다.

  ```toml
  [update]
  check = false
  ```

  끈 뒤에는 사용자가 `obit update --check`를 실행할 때만 확인합니다.

## 설치 및 다운로드

- Scoop, winget, 설치 스크립트는 [GitHub Releases](https://github.com/woodfishhhh/obituary/releases)에서 압축 파일을 내려받습니다. [GitHub 개인정보처리방침](https://docs.github.com/site-policy/privacy-policies/github-general-privacy-statement)이 적용됩니다.
- 미러(`-Mirror cn`)를 사용하면 `cn.obituary.woodfish.site`에서 내려받습니다. 서버는 문제 진단을 위해 일반적인 접속 로그(시각, IP 주소, 요청 URL)를 보관합니다.
- 앱 안의 업데이트(창의 ‘Update now’ 또는 ‘Download and restart to update’, `obit update --apply`)는 릴리스 아카이브와 `checksums.txt`를 GitHub Releases에서 내려받으며, GitHub가 10초 안에 응답하지 않으면 `cn.obituary.woodfish.site`에서 내려받습니다. 요청에는 IP 주소와 Obituary 버전을 담은 User-Agent만 포함되며 사용 데이터는 보내지 않습니다.
- 릴리스 시 GitHub Actions가 파일을 미러로 전송합니다. 미러는 파일을 직접 검증하고 제공하며 사용자의 다운로드 요청을 GitHub로 전달하지 않습니다.

## 에이전트 연결

`obit init`은 변경 계획을 먼저 보여 준 뒤 사용자 컴퓨터에 있는 에이전트의 설정 파일만 변경합니다. 에이전트 공급업체나 다른 곳으로 아무 정보도 보내지 않습니다.

## 웹사이트

- <https://obituary.woodfish.site/>에는 방문 분석이나 추적 스크립트가 없습니다. 중국 본토에서는 자체 서버, 그 밖의 지역에서는 GitHub Pages가 사이트를 제공하며 양쪽 모두 일반 접속 로그를 남깁니다.
- 웹사이트에 남긴 이메일 주소는 새 플랫폼과 버전을 알리는 데만 쓰입니다. 자체 서버에 보관하며 제삼자에게 제공하지 않습니다. 모든 메일에 수신 거부 링크가 있습니다.

## 문의

개인정보 관련 문의는 [GitHub 이슈](https://github.com/woodfishhhh/obituary/issues)에 남겨 주세요. 보안 문제는 [SECURITY.md](https://github.com/woodfishhhh/obituary/blob/main/SECURITY.md)에 따라 비공개로 신고해 주세요.
