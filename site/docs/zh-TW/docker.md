# Docker 容器與資料卷

在 agent 工作階段中啟動的容器也可以設定壽命。

```powershell
obit docker add web-1 3d       # 按名字或 ID 登记一个容器，三天后到期
obit docker list               # 登记了哪些容器（--json 可用）
obit resurrect <编号>          # 到期被停下的容器重新启动
```

## 為資料卷設定壽命或立即下葬

資料卷不會自動登記。訃告只處理你明確登記或下葬的資料卷，不會刪除映像檔或網路。

執行 `obit mark --docker-volume cache 3d`（也可以用 `add`）讓 `cache` 在三天後下葬；要立即下葬，請執行 `obit bury --docker-volume -- cache`。

Docker 必須使用 Linux 容器模式。僅支援本機範圍、採用 `local` 驅動程式且沒有額外選項的資料卷。首次使用時，若本機沒有輔助映像檔，Docker 會下載固定版本的 Debian 映像檔。

墓地必須有足夠空間存放完整封存檔。訃告先封存並驗證，再刪除資料卷；封存失敗就不刪。只要還有容器參照資料卷（即使容器已停止），或路徑與 bind 掛載重疊，訃告就不會動它。

`obit resurrect <編號>` 以原名重建資料卷，並永久保留 `io.obituary.restore` 標籤。若同名資料卷已存在，即使是空的也不覆寫。

訃告在刪除前後核對 Docker 事件。若無法確認已刪除，請保留驗證過的封存檔並查看 `obit doctor`，不要視為下葬成功。 每條 `docker-volume-recovery` 警告對應一筆待核實的記錄。請依提示在原 Docker 常駐程式上檢查卷；未提交的封存檔不能用 `obit resurrect` 或 `obit purge` 處理。

刪除防護能辨識 `docker volume rm`、`docker volume remove`、`docker volume prune`、`docker compose down -v`（或 `--volumes`）及 `docker system prune --volumes`。只有直接以 `docker volume rm` 指定卷名時才建議改用 `obit bury --docker-volume`，不會猜測批次清理的目標。

## 到期只停止容器

- 執行 `docker stop` 後，容器和它的可寫層都會保留。`obit resurrect <编号>` 會執行 `docker start`，並停止追蹤容器的壽命。
- 保留期（`[graveyard] retention_days`，預設 7 天）結束後才會用 `docker rm` 移除容器，絕不帶 `-v` 或 `-f`。
- 如果你在保留期內啟動過容器，即使後來又停止了它，訃告也會保留它並停止追蹤。舊版停止的容器沒有啟動時間可供核對；訃告不會刪除，你仍可用 `obit resurrect <編號>` 復活。
- 即使用 `obit purge` 或 `--all`，也要等容器的保留期結束。資源回收筒模式不適用於容器：仍是先停止，保留期過後才移除。

## 以完整 ID 識別容器

- 名字只用於顯示；每次停、啟、刪之前都重新核對完整 ID 和建立時間，對不上就不動。
- 訃告會記錄容器所屬的 Docker context 和 daemon，往後只對該 daemon 操作，切換 `docker context` 不會改變操作對象。
- 容器被你在訃告之外刪掉了，訃告會把它記成「外部移除」並清掉記錄。
- 設了 `--rm`（AutoRemove）的容器停下就會被 Docker 自己刪掉，訃告拒絕登記它。

## 自動登記工作階段中的容器

自動登記預設關閉；此時訃告不會尋找容器，你仍可用 `obit docker add` 自行登記。開啟後，訃告會在工作階段結束時，從背景尋找這段期間建立的容器，只納入 Compose 工作目錄或繫結掛載位於專案內的容器。其他容器可能是你在別處啟動的，不會受影響。符合條件的容器會登記 3 天壽命，事後通知你；用 `obit spare <编号>` 取消後，同一工作階段不會再次登記該容器。如果工作階段不在 Git 專案內（例如從家目錄啟動），或有多個工作階段認領同一個容器，就無法確定是誰啟動的，只會列為提議等你核准。設為 `[proposals] mode = "ask"` 時也會先提出建議，執行 `obit approve` 後才開始追蹤。

每個容器最多只會產生一項提議。Hook 不會呼叫 Docker；如果未安裝或未啟動 Docker，訃告會直接略過，`obit doctor` 可查看原因。Docker 未執行時不能核准容器提議，`obit proposals` 和小視窗的「待你決定」頁都會說明；啟動 Docker 後即可核准。
