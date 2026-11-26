# Docker 컨테이너와 볼륨

에이전트 세션에서 시작된 컨테이너도 수명을 가질 수 있습니다.

```powershell
obit docker add web-1 3d       # register a container by name or ID; it expires in three days
obit docker list               # registered containers (--json works too)
obit resurrect <id>            # start a container that was stopped at expiry
```

## 컨테이너를 시작할 때 바로 수명 등록하기

임시 컨테이너는 처음 띄울 때 수명을 정해 두지 않으면 가장 쉽게 방치됩니다. `docker run -d`는 전체 컨테이너 ID를 표준 출력으로 돌려주므로 그대로 `obit docker add`에 넘길 수 있습니다.

```powershell
# PowerShell
$cid = docker run -d --name tmp-redis redis:7
if ($LASTEXITCODE -eq 0) { obit docker add $cid 4h }
```

```bash
# Bash / zsh (no name required; pass the returned ID directly)
cid=$(docker run -d redis:7) && obit docker add "$cid" 4h
```

`obit docker add <name-or-id> <lifespan>`은 컨테이너 이름, 짧은 ID 또는 64자리 전체 ID를 받으며 `--at`이나 `--note` 옵션은 지원하지 않습니다. 등록하면 장부 번호와 만료 시각(`#<id> docker:<name> (<12-char-id>)  expires <time>`)을 출력합니다. `docker run`에 `--rm`을 붙이지 마세요. AutoRemove 컨테이너는 중지 즉시 Docker가 삭제하므로 `obit docker add`가 등록을 거부합니다. 만료 전에 기간을 늘리거나 추적을 해제하려면 `obit docker list`나 `obit list`의 번호로 `obit extend <id> <lifespan>` 또는 `obit spare <id>`를 실행하세요.

## 보관 기간 동안 중지된 컨테이너 확인하고 부활하기

컨테이너가 만료되어 `docker stop`이 실행되면 활성 목록인 `obit docker list`에서 빠져 묘지로 옮겨지며 `[graveyard] retention_days`(기본 7일) 동안 보관됩니다. 보관 기간 중 중지된 컨테이너를 확인하거나 다시 살리려면 아래 명령을 사용합니다.

```powershell
obit graveyard                 # list all restorable burials; containers show as docker:<name> (<12-char-id>) with purge countdown
obit graveyard --json          # machine-readable output; filter burials where kind == "container"
obit stats                     # total graveyard count and the next purge time
obit resurrect <id>            # start the stopped container by its numeric graveyard ID and stop tracking it
```

`obit docker add`로 직접 등록한 컨테이너는 프로젝트 디렉터리에 연결되지 않으므로 `obit graveyard --here`에는 나오지 않습니다. `--here`는 프로젝트 자동 등록이나 제안 승인으로 들어온 컨테이너만 일치합니다. 데스크톱에서는 트레이 메뉴에서 묘지 총개수와 최근 매장된 항목 최대 10개(클릭하면 바로 부활)를 볼 수 있고, 창의 묘지(Graveyard) 페이지에서는 이름으로 검색하고 남은 보관 시간을 확인한 뒤 선택해 부활할 수 있습니다. 백그라운드 실행기는 컨테이너가 중지되거나 영구 삭제를 앞둘 때 먼저 시스템 알림을 보내지 않으므로, 보관 기간이 끝나기 전에 살펴보려면 `obit graveyard`나 창의 묘지 페이지를 확인하세요.

## 볼륨에 수명을 지정하거나 바로 매장하기

볼륨은 자동으로 등록되지 않습니다. 직접 등록하거나 매장하도록 지정한 볼륨만 처리하며 이미지와 네트워크는 삭제하지 않습니다.

`obit mark --docker-volume cache 3d`(`add`도 가능)를 실행하면 `cache`를 3일 뒤에 매장합니다. 지금 매장하려면 `obit bury --docker-volume -- cache`를 실행하세요.

Docker가 Linux 컨테이너 모드여야 합니다. 추가 옵션 없이 `local` 드라이버를 사용하는 로컬 범위의 볼륨만 지원합니다. 처음 사용할 때 보조 이미지가 없으면 고정 버전의 Debian 이미지를 받습니다.

묘지에 볼륨 전체를 보관할 공간이 필요합니다. 아카이브를 만들고 검증한 뒤에만 볼륨을 삭제합니다. 중지된 컨테이너라도 볼륨을 참조하거나 볼륨 경로가 바인드 마운트와 겹치면 그대로 둡니다.

`obit resurrect <ID>`는 원래 이름으로 볼륨을 다시 만들며, `io.obituary.restore` 레이블이 영구적으로 남습니다. 같은 이름의 볼륨이 이미 있으면 비어 있어도 덮어쓰지 않습니다.

삭제 전후에 Docker 이벤트를 대조합니다. 삭제를 확인할 수 없다면 검증된 아카이브를 보관하고 `obit doctor`를 확인하세요. 매장이 성공했다고 간주하면 안 됩니다. 확인이 필요한 항목마다 `docker-volume-recovery` 경고가 표시됩니다. 안내에 따라 원래 Docker 데몬에서 볼륨을 확인하세요. 장부에 기록되지 않은 아카이브에는 `obit resurrect`나 `obit purge`를 사용할 수 없습니다.

삭제 가드는 `docker volume rm`, `docker volume remove`, `docker volume prune`, `docker compose down -v`(또는 `--volumes`), `docker system prune --volumes`를 인식합니다. 단순한 `docker volume rm` 명령에 볼륨 이름이 명시된 경우에만 `obit bury --docker-volume`를 제안합니다.

## 만료되면 먼저 중지합니다

- `docker stop`을 실행해도 컨테이너와 쓰기 가능한 레이어는 그대로 남습니다. `obit resurrect <id>`는 `docker start`로 컨테이너를 시작하고 수명 추적을 끝냅니다.
- 보관 기간(`[graveyard] retention_days`, 기본 7일)이 지난 뒤에만 `docker rm`으로 컨테이너를 제거합니다. 이때 `-v`나 `-f`는 사용하지 않습니다.
- 보존 기간에 컨테이너를 시작하면 나중에 다시 멈춰도 삭제하지 않고 수명 추적을 끝냅니다. 이전 버전이 멈춘 컨테이너에는 비교할 시작 시각이 없으므로 삭제하지 않습니다. `obit resurrect <id>`로 다시 시작할 수 있습니다.
- `obit purge`나 `--all`도 컨테이너의 보관 기간을 건너뛸 수 없습니다. 휴지통 모드는 컨테이너에 적용되지 않습니다. 먼저 중지하고 보관 기간이 지난 뒤 제거합니다.

## 전체 ID로 식별합니다

- 이름은 표시용입니다. 중지, 시작, 제거하기 전에 매번 전체 ID와 생성 시각을 다시 확인하고 기록과 다르면 아무 작업도 하지 않습니다.
- 컨테이너의 Docker 컨텍스트와 데몬을 기록하고 해당 데몬에서만 작업합니다. 따라서 `docker context`를 바꿔도 다른 데몬에 명령이 전달되지 않습니다.
- Obituary 밖에서 컨테이너를 제거하면 외부에서 제거한 사실을 기록하고 추적 목록에서 지웁니다.
- `--rm`(AutoRemove)으로 시작한 컨테이너는 중지하면 Docker가 바로 삭제하므로 등록을 거부합니다.

## 세션에서 만든 컨테이너 자동 등록

기본값인 자동 등록 끄기 상태에서는 컨테이너를 찾지 않습니다. 개별 등록에는 `obit docker add`를 사용합니다. 자동 등록을 켜면 세션 종료 시 해당 세션 동안 생성된 컨테이너를 백그라운드에서 찾습니다. Compose 작업 디렉터리나 바인드 마운트가 프로젝트 안에 있는 컨테이너만 대상입니다. 나머지는 다른 곳에서 시작했을 수 있으므로 그대로 둡니다. 찾은 컨테이너는 수명을 3일로 등록하고 나중에 알려 줍니다. `obit spare <id>`로 해제하면 그 세션에서는 같은 컨테이너를 다시 등록하지 않습니다. Git 프로젝트 밖에서(예: 홈 디렉터리에서) 세션을 시작했거나 여러 세션이 같은 컨테이너를 자기 것으로 인식하면 어느 세션에서 시작했는지 알 수 없어 제안만 하고 승인을 기다립니다. `[proposals] mode = "ask"`인 경우에도 제안만 만들고 `obit approve` 후에 추적합니다.

컨테이너당 제안은 최대 하나입니다. 훅은 Docker를 직접 호출하지 않습니다. Docker가 설치되지 않았거나 실행 중이 아니면 알림 없이 건너뛰며 `obit doctor`에서 이유를 확인할 수 있습니다. Docker가 중지된 동안은 컨테이너 제안을 승인할 수 없습니다. `obit proposals`와 창의 ‘For you’ 페이지에 이 상태가 표시되며 Docker를 시작한 뒤 승인할 수 있습니다.
