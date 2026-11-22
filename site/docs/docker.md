# Docker 容器和卷

agent 会话里起的容器，也可以有寿命。

```powershell
obit docker add web-1 3d       # 按名字或 ID 登记一个容器，三天后到期
obit docker list               # 登记了哪些容器（--json 可用）
obit resurrect <编号>          # 到期被停下的容器重新启动
```

## 给卷设寿命，或立即下葬

卷不会自动登记。讣告只处理你明确登记或下葬的卷，不删除镜像和网络。

运行 `obit mark --docker-volume cache 3d`（也可用 `add`）让 `cache` 三天后下葬；要立即下葬，运行 `obit bury --docker-volume -- cache`。

Docker 必须使用 Linux 容器模式。只支持本地作用域、使用 `local` 驱动且没有额外选项的卷。首次使用时，如果本地还没有辅助镜像，Docker 会拉取固定版本的 Debian 镜像。

请给墓地留够存放完整归档的空间。讣告先归档、校验，再删卷；归档失败就不删。只要还有容器引用这个卷（即使容器已停止），或卷路径与 bind 挂载路径重叠，讣告就不会动它。

`obit resurrect <编号>` 按原名重建卷，重建的卷会永久带有 `io.obituary.restore` 标签。原名已被占用时，即使同名卷是空的也不会覆盖。

讣告在删除前后核对 Docker 事件。如果无法确认卷已删除，请保留已校验的归档，查看 `obit doctor` 的诊断，不要当作下葬成功。

删除护栏能识别 `docker volume rm`、`docker volume remove`、`docker volume prune`、`docker compose down -v`（或 `--volumes`）和 `docker system prune --volumes`。只有直接用 `docker volume rm` 指定卷名时，才建议改用 `obit bury --docker-volume`；不会猜测批量清理会删哪些卷。

## 到期 = 停下

- `docker stop` 之后容器和它的可写层都还在，`obit resurrect <编号>` 就是 `docker start`，复活后不再跟踪它的寿命。
- 保留期（`[graveyard] retention_days`，默认 7 天）过后才 `docker rm`，不带 `-v`、不带 `-f`。
- 那时容器要是在运行（被你或别人手动启动过），就当作复活，不删。
- `obit purge` 和 `--all` 对容器同样要等保留期。回收站模式对容器不适用，照样是先停下、保留期后删除。

## 按完整 ID 认容器

- 名字只用于显示；每次停、启、删之前都重新核对完整 ID 和创建时间，对不上就不动。
- 讣告记住容器所在的 Docker 上下文和 daemon，之后只对那一个 daemon 操作；你切换了 `docker context` 也不会动到别处的同名容器。
- 容器被你在讣告之外删掉了，讣告会把它记成「外部移除」并清掉记录。
- 设了 `--rm`（AutoRemove）的容器停下就会被 Docker 自己删掉，讣告拒绝登记它。

## 打开自动登记后，会话里新起的容器也会登记

自动登记关闭（默认）时讣告不找容器，用上面的 `obit docker add` 登记。打开后，agent 会话结束时，讣告在后台找出这个会话期间新建的容器：只认 compose 的工作目录或绑定挂载在项目里的；归不到项目的可能是你在别处起的，不碰。认出来的直接登记 3 天寿命，事后告诉你；`obit spare <编号>` 撤回后，这个会话不会再登记它。会话不在 Git 项目里（比如在用户目录里开的会话），或者同一个容器同时被几个会话认领，说不清是谁起的，就只列成提议等你批准。设了 `[proposals] mode = "ask"` 时改为提议，你 `obit approve` 才登记。

每个容器只出一条提议。hook 里从不调用 docker；没装 Docker 或 Docker 没启动时静默跳过，`obit doctor` 会说明。Docker 没启动时容器提议不能批准，`obit proposals` 和小窗口「待你决定」会直说，启动 Docker 后再批准。
