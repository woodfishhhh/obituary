# 参与讣告

[English](#contributing-to-obituary)

欢迎提 issue 和 PR。最欢迎的是带着修复来的 issue：发现问题，顺手修好，一起提上来。合进去之后，你的头像会出现在 README 和官网的贡献者墙上。

## 提 issue

- **Bug**：写清楚怎么复现、你以为会怎样、实际怎样，附上 `obit --version` 和 `obit doctor` 的输出。贴之前看一眼，删掉不想公开的路径。
- **带着修复**：直接开 PR，描述里写 `Fixes #<issue 编号>`。还没有 issue 也可以直接开 PR，把问题写在描述里。
- **想法和新功能**：先开 issue 聊聊方向，免得白写。
- **安全问题**（可能误删、移错文件、越权）：不要公开提，见 [SECURITY.md](SECURITY.md)。

## 改代码

- 需要 Windows 和 Go 1.26+。构建命令见 README 的[构建](README.md#构建)。
- 先读 [AGENTS.md](AGENTS.md)，那里是代码约定。改动跨模块时再读 [docs/architecture.md](docs/architecture.md)。
- 动到移动、下葬、删除的代码时，先写一个能证明不会误删的失败测试，再修。
- 提交前跑：

  ```bash
  gofmt -l .
  go vet ./...
  golangci-lint run ./...
  go test -short ./...
  ```

  全量测试和变异测试在 CI 上跑。
- 改小窗口的界面：在 `cmd/obit-gui/frontend` 里 `pnpm install`、`pnpm dev` 预览，`pnpm build` 更新 `dist/`，`dist/` 要一起提交。
- 提交信息用英文，一句话说清改了什么，例如 `fix: resurrect keeps the original name when the folder is empty`。

## 许可

提交的代码按 [MIT](LICENSE) 许可发布。

---

# Contributing to Obituary

Issues and pull requests are welcome. The most welcome is an issue that comes with its fix: you found it, you fixed it, send both. Once it is merged, your avatar joins the contributor wall in the README and on the website.

## Issues

- **Bugs**: how to reproduce it, what you expected, what happened, plus the output of `obit --version` and `obit doctor`. Remove any paths you don't want public before posting.
- **With a fix**: open a pull request and write `Fixes #<issue number>` in it. No issue yet? Open the pull request anyway and describe the problem there.
- **Ideas and features**: open an issue first so we can agree on the direction before you write code.
- **Security problems** (anything that could delete or move the wrong file): don't post them publicly; see [SECURITY.md](SECURITY.md).

## Code

- You need Windows and Go 1.26+. Build commands are in the README under [Build](README.en.md#build).
- Read [AGENTS.md](AGENTS.md) first for the code conventions, and [docs/architecture.md](docs/architecture.md) when a change crosses modules.
- When you touch code that moves, buries or deletes, first write a failing test that proves nothing wrong gets deleted, then fix it.
- Before you send it:

  ```bash
  gofmt -l .
  go vet ./...
  golangci-lint run ./...
  go test -short ./...
  ```

  The full suite and the mutation tests run in CI.
- For the window's interface: in `cmd/obit-gui/frontend`, `pnpm install`, `pnpm dev` to preview, `pnpm build` to update `dist/`, and commit `dist/` too.
- Commit messages in English, one line saying what changed, e.g. `fix: resurrect keeps the original name when the folder is empty`.

## Licence

Contributions are released under the [MIT](LICENSE) licence.
