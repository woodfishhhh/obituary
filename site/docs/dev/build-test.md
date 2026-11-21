# 三平台构建和测试

使用 `go.mod` 指定的 Go 1.26.4 或更高版本。下面的命令从仓库根目录执行。先编译 CLI 和后台进程；指定多个包而不指定输出路径只检查编译，不安装、不启动服务。

```bash
go build ./cmd/obit ./cmd/obitd
go test -short ./internal/core/lifespan
```

Windows 需要可运行的文件时，分别构建两个程序。后台程序使用 GUI 子系统，避免唤醒时弹出控制台。

```bash
go build -o bin/obit.exe ./cmd/obit
go build -ldflags=-H=windowsgui -o bin/obitd.exe ./cmd/obitd
```

Linux 和 macOS 的 CLI 不依赖桌面库。在 Bash 中可以用 `GOOS=darwin GOARCH=arm64 go build ./cmd/obit ./cmd/obitd` 检查目标平台是否编得过；交叉编译不能验证通知、菜单栏或首次打开应用。桌面程序的原生条件见[小窗口](./gui)。

## 提交前检查

提交前在仓库根目录运行：

```bash
gofmt -l .
go vet ./...
golangci-lint run ./...
go test -short ./...
```

本地对改动包跑完整测试。全仓完整测试和变异测试在 CI 上跑；MCP 和删除护栏变异检查由定时或手动工作流分片运行。涉及文件移动、下葬或删除时，见[安全模型](./safety)。

网站与 Go 构建独立。在 `site/` 使用工作区固定的 pnpm 版本安装依赖，再分别验证两个部署目标：

```bash
pnpm install --frozen-lockfile
pnpm build
pnpm build:cn
```
