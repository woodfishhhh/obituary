# 性能审计的复测夹具

基线为 `7b8c120476162293addc97d87f74640157c7de2e`，Windows 11 build 26200、Ryzen 9 7845HX、24 逻辑线程、Go 1.26.11。`testdata/audit-6926/` 是实验源码，不参与产品构建；`evidence/` 是当轮观测值，已去掉本机目录。没有产品安装程序、真实账本或用户配置。

## 先看哪份证据

|要复验的事|入口|结果|
|---|---|---|
|单 hook 进程内、进程启动至退出|原有 `BenchmarkClaudeHook/BenchmarkCodexHook`，`driver.py hooks`|`inproc.txt`、`qpc.txt`、`hooks.txt`|
|真实 4/8/16 个 Post 与一个 `list`|`driver.py conc`（100/0）、`driver.py conc large`（1k/1k）|`conc.txt`、`conc-large.txt`；`delayed` 是到各波结束仍待回放的写入数|
|1k/10k 的 CLI、写锁等待与对账|`driver.py cli`，之后 `detail.py inner`|`cli.txt`、`inner.txt`；对账关闭 Explorer 展示回调，`reap-one`、`purge-one` 为实际操作|
|进程初始化、按需帮助/词表|`driver.py cost`、`detail.py paths`、`detail.py loader`|`cost2.txt`、`paths.txt`、`loader.txt`；DLL 调试事件不是独占 CPU 时间|
|常驻进程与身份定位并发|`check.py resident`、`extra.py res`、`scan.go`|`resident.txt` 是存活进程的工作集/CPU；`res2.txt` 只用 `probe/` 的突发和唤醒，不用其中受旧 stop 文件影响的 `obitd/` 数字；`scan.txt` 以 1/2/4/8 工人逐项核对位置|
|命名管道 + 行事务 + 断回包重放|`sql/main.go`、`extra.py sql`|`sql2.txt`、`commit-{single,batch}.txt`；每模式 609 次正常写，加 1 次丢回包、杀进程后同 ID 重试|
|GUI 首屏|`gui.py`，只在私有 Xvfb、私有 D-Bus、软件渲染下运行|`gui2.txt` 与 `gui-{0,10}.png`。窗口映射和非空内容分开；不是 Windows WebView2 数字。|

在 1k 活跃项、1k 墓地记录的真实 16 进程场景，hook、整波、读者的 p95 分别为 **1,049、1,272、469 ms**。SQLite 微原型的 16 客户端短批 ACK p95 为 **8.8 ms**，只说明行写、同步和 IPC 的成本，不能当完整 hook 的数字。`evidence/raw` 的每一行都是 `[名称, 样本数组]`；重复运行的同名组按时间追加。以对应 `.txt` 的成功轮次为准；`cost` 和 `sql` 的早期不完整数据不能同成功轮次混算。

## Windows 复测

在基线提交的一次性工作树执行。以下是 **Git Bash** 命令，需要 Windows 原生 `go.exe`、`python.exe` 和 Git。`.obit/tmp/audit-6926` 和驱动打印的 `%TEMP%/obit-audit-6926-*` 必须是本次独占的新目录；后者由脚本位置推导，旧目录会污染结果。驱动对派生进程设置独立的用户、账本和配置目录，禁用遥测和更新。不要手工给探针传实际用户目录。一次只跑一组重负载。

```bash
mkdir -p .obit/tmp
test ! -e .obit/tmp/audit-6926 || exit 1
cp -R experiments/perf-audit/testdata/audit-6926 .obit/tmp/
b=.obit/tmp/audit-6926
go.exe build -ldflags='-s -w' -o "$b/obit.exe" ./cmd/obit
go.exe build -ldflags='-s -w -H=windowsgui' -o "$b/obitd.exe" ./cmd/obitd
go.exe build -tags desktop,production -ldflags='-s -w -H=windowsgui' -o "$b/obit-gui.exe" ./cmd/obit-gui
for name in probe cli-inner scan fast empty; do
  go.exe build -ldflags='-s -w' -o "$b/$name.exe" "$b/$name.go" || exit 1
done
go.exe test -c -o "$b/cachehost.exe" ./internal/i18n
(cd "$b/sql" && go.exe build -o ../sql.exe .)
```

只重测需要的部分。例如 `python.exe "$b/extra.py" sql` 启动夹具自己的管道服务、验证回执和重放，然后结束它。`python.exe "$b/driver.py" hooks` 检查原生 hook 回执和账本副作用；`hooks` 的 v17 断言和 `check.py fence` 的 v18 输入只适用于这次基线。测按需词条时先 `python.exe "$b/extra.py" build` 生成临时 overlay，再运行 `detail.py paths`；`check.py qpc` 用同一测试缓存服务复测亚毫秒 Pre。Windows loader 探针需 MinGW，命令为 `gcc.exe -municode -O2 "$b/load.c" -o "$b/load.exe"`。

Linux GUI 需另编 `-tags desktop,production,webkit2_41` 的 `obit-gui-linux` 放在脚本旁。按 `scripts/test-linux-desktop-session.sh` 的隔离方式，设置新的 `HOME/OBIT_HOME/OBIT_CONFIG_DIR/XDG_RUNTIME_DIR`，取消继承 `DISPLAY/WAYLAND_DISPLAY/DBUS_SESSION_BUS_ADDRESS`，再由 `xvfb-run -a` 和 `dbus-run-session` 执行 `gui.py`；脚本用 X11 窗口内容区像素和截图判定首屏。不要在真实桌面上运行。

`evidence/legacy.txt` 来自归档源码 `obituary-archive@2ae9c91` 的测试，不是主线已有 SQLite 迁移的证明。这些产物只用于实验，不是安装脚本或发布实现。
