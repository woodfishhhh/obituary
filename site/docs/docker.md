# Docker 容器

agent 会话里起的容器，也可以有寿命。

```powershell
obit docker add web-1 3d       # 按名字或 ID 登记一个容器，三天后到期
obit docker list               # 登记了哪些容器（--json 可用）
obit resurrect <编号>          # 到期被停下的容器重新启动
```

## 只管容器

讣告只管容器。卷里是数据，删卷就是永久删除，所以卷、镜像和网络一律不碰。

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

## 会话里新起的容器直接登记

agent 会话结束后，讣告在后台找出这个会话期间新建的容器：只认 compose 的工作目录或绑定挂载在项目里的；归不到项目的可能是你在别处起的，不碰。认出来的直接登记 3 天寿命，事后告诉你；`obit spare <编号>` 撤回后，这个会话不会再登记它。设了 `[proposals] mode = "ask"` 时改为提议，你 `obit approve` 才登记。

hook 里从不调用 docker；没装 Docker 或 Docker 没启动时静默跳过，`obit doctor` 会说明。
