# 解除安裝

```powershell
obit uninstall --dry-run      # 先看会撤掉什么
obit uninstall                # 撤掉讣告装过的所有东西，确认后执行
.\install.ps1 -Uninstall      # 用安装脚本装的：再删掉程序文件、PATH 和开始菜单快捷方式
```

`obit uninstall` 只會移除訃告自己加上的項目：使用者層級和目前 Git 專案的 agent hook、MCP 與 Skill，以及登入時自動啟動的設定、排程工作、執行中的背景程式、右鍵選單、檔案總管中的日期標籤和資料夾策略。

## 預設保留墓地和帳本

預設會保留墓地和帳本，並顯示墓地的位置與存放量。確定要一併刪除時才加上 `--purge-data`；訃告會先列出墓地內容，再請你確認一次。

## 其他專案的 hook

如果其他專案也安裝了專案層級的 hook，請先在那些專案裡分別執行 `obit uninstall`，再移除程式。否則這些 hook 會因找不到 `obit` 而失效。

## Scoop 和 winget

先執行 `obit uninstall`，再用各自的解除安裝命令。
