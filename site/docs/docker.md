# Docker 容器和卷

agent 会话里起的容器，也可以有寿命。

```powershell
obit docker add web-1 3d       # 按名字或 ID 登记一个容器，三天后到期
obit docker list               # 登记了哪些容器（--json 可用）
obit resurrect <编号>          # 到期被停下的容器重新启动
```

## 起容器时顺手登记寿命

临时容器最容易忘关，起的时候直接把寿命写上最省事。`docker run -d` 会把完整容器 ID 打到标准输出，接住它立刻交给 `obit docker add` 就行：

```powershell
# PowerShell
$cid = docker run -d --name tmp-redis redis:7
if ($LASTEXITCODE -eq 0) { obit docker add $cid 4h }
```

```bash
# Bash / zsh（不用手动起名，直接传返回的 ID）
cid=$(docker run -d redis:7) && obit docker add "$cid" 4h
```

`obit docker add <容器名或 ID> <时长>` 接收容器名、短 ID 或完整 64 位 ID，不支持 `--at` 或 `--note`；登记后会输出账本编号和到期时间（格式为 `#<编号> docker:<名字> (<12位短ID>)  到期 <时间>`）。起临时容器时不要加 `docker run --rm`：带 `--rm` 的容器停下就会被 Docker 直接删掉，讣告会拒绝登记它。到期前要续命或取消跟踪，按 `obit docker list` 或 `obit list` 里的编号运行 `obit extend <编号> <时长>` 或 `obit spare <编号>`。

## 保留期内查看和捞回已停止容器

容器到期被 `docker stop` 后，就会从 `obit docker list` 转进墓地，进入保留期（`[graveyard] retention_days`，默认 7 天）。`obit docker list` 只列尚未到期的活跃容器；要翻看保留期里待清理的已停止容器，用下面几个入口：

```powershell
obit graveyard                 # 查看全部保留期条目，容器显示为 docker:<名字> (<12位短ID>) 及火化倒计时
obit graveyard --json          # 机器可读输出，可按 burials 中 kind == "container" 筛选
obit stats                     # 查看墓地总数和下一次火化时间
obit resurrect <编号>          # 按墓地里的数字编号执行 docker start，并结束寿命跟踪
```

手动用 `obit docker add` 登记的容器不绑定项目目录，不会出现在 `obit graveyard --here` 里，只有通过项目自动登记或提议批准的容器才会按当前目录过滤出来。桌面端也可以在托盘菜单里看当前墓地总数和「最近下葬的」（最多最近 10 条，点一条直接复活），或打开小窗口的「墓地」页按名字搜索、查看剩余倒计时并勾选复活。后台在容器停下或临近火化前不会主动弹系统通知，想在 7 天内定期清理或捞回，直接看 `obit graveyard` 或小窗口墓地页。

## 给卷设寿命，或立即下葬

卷不会自动登记。讣告只处理你明确登记或下葬的卷，不删除镜像和网络。

运行 `obit mark --docker-volume cache 3d`（也可用 `add`）让 `cache` 三天后下葬；要立即下葬，运行 `obit bury --docker-volume -- cache`。

Docker 必须使用 Linux 容器模式。只支持本地作用域、使用 `local` 驱动且没有额外选项的卷。首次使用时，如果本地还没有辅助镜像，Docker 会拉取固定版本的 Debian 镜像。

请给墓地留够存放完整归档的空间。讣告先归档、校验，再删卷；归档失败就不删。只要还有容器引用这个卷（即使容器已停止），或卷路径与 bind 挂载路径重叠，讣告就不会动它。

`obit resurrect <编号>` 按原名重建卷，重建的卷会永久带有 `io.obituary.restore` 标签。原名已被占用时，即使同名卷是空的也不会覆盖。

讣告在删除前后核对 Docker 事件。如果无法确认卷已删除，请保留已校验的归档，查看 `obit doctor` 的诊断，不要当作下葬成功。 `docker-volume-recovery` 的每条 WARN 对应一个待核实条目。按提示在原 Docker 守护进程上检查卷；未提交的归档不能用 `obit resurrect` 或 `obit purge` 处理。

删除护栏能识别 `docker volume rm`、`docker volume remove`、`docker volume prune`、`docker compose down -v`（或 `--volumes`）和 `docker system prune --volumes`。只有直接用 `docker volume rm` 指定卷名时，才建议改用 `obit bury --docker-volume`；不会猜测批量清理会删哪些卷。

## 到期 = 停下

- `docker stop` 之后容器和它的可写层都还在，`obit resurrect <编号>` 就是 `docker start`，复活后不再跟踪它的寿命。
- 保留期（`[graveyard] retention_days`，默认 7 天）过后才 `docker rm`，不带 `-v`、不带 `-f`。
- 如果你在保留期内启动过容器，即使后来又停了它，讣告也会保留它，不再跟踪。旧版本停下的容器没有启动时间可供核对；讣告不删除它，你仍可用 `obit resurrect <编号>` 复活。
- `obit purge` 和 `--all` 对容器同样要等保留期。回收站模式对容器不适用，照样是先停下、保留期后删除。

## 按完整 ID 认容器

- 名字只用于显示；每次停、启、删之前都重新核对完整 ID 和创建时间，对不上就不动。
- 讣告记住容器所在的 Docker 上下文和 daemon，之后只对那一个 daemon 操作；你切换了 `docker context` 也不会动到别处的同名容器。
- 容器被你在讣告之外删掉了，讣告会把它记成「外部移除」并清掉记录。
- 设了 `--rm`（AutoRemove）的容器停下就会被 Docker 自己删掉，讣告拒绝登记它。

## 打开自动登记后，会话里新起的容器也会登记

自动登记关闭（默认）时讣告不找容器，用上面的 `obit docker add` 登记。打开后，agent 会话结束时，讣告在后台找出这个会话期间新建的容器：只认 compose 的工作目录或绑定挂载在项目里的；归不到项目的可能是你在别处起的，不碰。认出来的直接登记 3 天寿命，事后告诉你；`obit spare <编号>` 撤回后，这个会话不会再登记它。会话不在 Git 项目里（比如在用户目录里开的会话），或者同一个容器同时被几个会话认领，说不清是谁起的，就只列成提议等你批准。设了 `[proposals] mode = "ask"` 时改为提议，你 `obit approve` 才登记。

每个容器只出一条提议。hook 里从不调用 docker；没装 Docker 或 Docker 没启动时静默跳过，`obit doctor` 会说明。Docker 没启动时容器提议不能批准，`obit proposals` 和小窗口「待你决定」会直说，启动 Docker 后再批准。
