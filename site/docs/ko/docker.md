# Docker 컨테이너

에이전트 세션에서 시작된 컨테이너도 수명을 가질 수 있습니다.

```powershell
obit docker add web-1 3d       # register a container by name or ID; it expires in three days
obit docker list               # registered containers (--json works too)
obit resurrect <id>            # start a container that was stopped at expiry
```

## 관리 대상은 컨테이너뿐입니다

Obituary는 컨테이너만 관리합니다. 데이터를 보관하는 볼륨은 삭제하면 복구할 수 없으므로 볼륨, 이미지, 네트워크는 건드리지 않습니다.

## 만료되면 먼저 중지합니다

- `docker stop`을 실행해도 컨테이너와 쓰기 가능한 레이어는 그대로 남습니다. `obit resurrect <id>`는 `docker start`로 컨테이너를 시작하고 수명 추적을 끝냅니다.
- 보관 기간(`[graveyard] retention_days`, 기본 7일)이 지난 뒤에만 `docker rm`으로 컨테이너를 제거합니다. 이때 `-v`나 `-f`는 사용하지 않습니다.
- 그때 사용자나 다른 사람이 이미 컨테이너를 시작해 실행 중이라면 부활한 것으로 보고 보존합니다.
- `obit purge`나 `--all`도 컨테이너의 보관 기간을 건너뛸 수 없습니다. 휴지통 모드는 컨테이너에 적용되지 않습니다. 먼저 중지하고 보관 기간이 지난 뒤 제거합니다.

## 전체 ID로 식별합니다

- 이름은 표시용입니다. 중지, 시작, 제거하기 전에 매번 전체 ID와 생성 시각을 다시 확인하고 기록과 다르면 아무 작업도 하지 않습니다.
- 컨테이너의 Docker 컨텍스트와 데몬을 기록하고 해당 데몬에서만 작업합니다. 따라서 `docker context`를 바꿔도 다른 데몬에 명령이 전달되지 않습니다.
- Obituary 밖에서 컨테이너를 제거하면 외부에서 제거한 사실을 기록하고 추적 목록에서 지웁니다.
- `--rm`(AutoRemove)으로 시작한 컨테이너는 중지하면 Docker가 바로 삭제하므로 등록을 거부합니다.

## 세션에서 만든 컨테이너 자동 등록

기본값인 자동 등록 끄기 상태에서는 컨테이너를 찾지 않습니다. 개별 등록에는 `obit docker add`를 사용합니다. 자동 등록을 켜면 세션 종료 시 해당 세션 동안 생성된 컨테이너를 백그라운드에서 찾습니다. Compose 작업 디렉터리나 바인드 마운트가 프로젝트 안에 있는 컨테이너만 대상입니다. 나머지는 다른 곳에서 시작했을 수 있으므로 그대로 둡니다. 찾은 컨테이너는 수명을 3일로 등록하고 나중에 알려 줍니다. `obit spare <id>`로 해제하면 그 세션에서는 같은 컨테이너를 다시 등록하지 않습니다. Git 프로젝트 밖에서(예: 홈 디렉터리에서) 세션을 시작했거나 여러 세션이 같은 컨테이너를 자기 것으로 인식하면 어느 세션에서 시작했는지 알 수 없어 제안만 하고 승인을 기다립니다. `[proposals] mode = "ask"`인 경우에도 제안만 만들고 `obit approve` 후에 추적합니다.

컨테이너당 제안은 최대 하나입니다. 훅은 Docker를 직접 호출하지 않습니다. Docker가 설치되지 않았거나 실행 중이 아니면 알림 없이 건너뛰며 `obit doctor`에서 이유를 확인할 수 있습니다. Docker가 중지된 동안은 컨테이너 제안을 승인할 수 없습니다. `obit proposals`와 창의 ‘For you’ 페이지에 이 상태가 표시되며 Docker를 시작한 뒤 승인할 수 있습니다.
