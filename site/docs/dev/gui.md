# 小窗口

小窗口没有第二份账本。`cmd/obit-gui` 把 `internal/gui` 的 API 绑定给 Wails；React 通过 `frontend/src/api.ts` 调用它。在浏览器开发模式里，同一个接口由内存 mock 实现。因此预览可以检查布局和交互，不能证明真实文件已经复活。

`App.tsx` 读取摘要，把刷新信号传给当前页面。通常按即将到期、墓地、决定、设置、日志五页导航；时间线外观把前两项合并。`catalog.ts` 按需加载八种发行外观；实验外观只进 playground。改一项操作要同时检查 Go 绑定、前端类型和 mock 的返回值。

| 平台 | 构建和验收 |
| --- | --- |
| Windows amd64 | `desktop,production` 标签和 Windows GUI 子系统；托盘及原生窗口要在 Windows 验证。 |
| Ubuntu 24.04 amd64 | 需要 GTK 3、WebKitGTK 4.1 开发包和 CGO；`linux-desktop.yml` 在独立 Xvfb 与会话总线内验证桌面包。 |
| macOS | `macos-desktop.yml` 在 macOS runner 上测试原生绑定，构建通用 app 和 DMG。ad-hoc 签名不等于公证。 |

前端源码在 `cmd/obit-gui/frontend`；改完需提交构建后的 `dist/`，供 Go 嵌入。平台打包和测试范围见[构建和测试](./build-test)。
