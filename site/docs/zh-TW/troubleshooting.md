# 疑難排解

先執行 `obit doctor`。它只讀檢查安裝、整合、背景服務與資料，不會修改檔案；需要更多資訊時加上 `--json` 或 `--verbose`。

## PATH 與校驗和

**症狀：** 終端機找不到 `obit`，或下載的壓縮檔校驗和不一致。

**處理：** 從同一版本下載 `checksums.txt`，在下載目錄執行 `sha256sum --check --strict checksums.txt`。Windows PowerShell 可用 `Get-FileHash .\obit-*.zip -Algorithm SHA256` 比對。校驗不一致時不要執行，修改 PATH 後重新開啟終端機。

## Windows、macOS 與 Linux 桌面

Windows SmartScreen 未簽署程式會顯示保護提示；先核對 SHA-256，再選「更多資訊 → 仍要執行」。小視窗缺少 WebView2 時安裝 Microsoft Edge WebView2 Runtime；Windows 小視窗目前只有 x64。

macOS app 未 Developer ID 簽署或公證。核對校驗和後第一次開啟，在「系統設定 → 隱私權與安全性」選「仍要開啟」。不要關閉 Gatekeeper，也不要放行顯示損壞或惡意軟體的 app。選單列未隨登入啟動時執行 `obit daemon status` 和 `launchctl list | grep obituary`。

Ubuntu 24.04 小視窗需要 GTK 3 和 WebKitGTK 4.1：

```bash
sudo apt update
sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0
obit doctor
```

用 `systemctl --user status obituary.timer`、`systemctl --user list-timers | grep obituary` 和 `crontab -l` 檢查排程。

## 背景、更新與檔案

背景未執行時查看 `obit daemon status`，立即處理可執行 `obit sweep`，恢復排程可執行 `obit daemon enable`。國內鏡像使用 `bash install.sh --mirror cn`，不可混用不同版本的壓縮檔與校驗檔。

WSL 掛載磁碟改名後先確認真實 Windows 路徑，不要用舊路徑強行復原。檔案被占用時讖告會保留原檔並重試；關閉占用程式後執行 `obit sweep`，或在視窗選擇「重試」或「停止追蹤」。TOML 拼寫、引號或時長錯誤會由 `obit config` 指出，未知鍵不會被忽略。

## 整合與容器

hook 沒有生效時執行 `obit init --show`、`obit init --dry-run` 和 `obit doctor --verbose`，並確認沒有設定 `OBIT_DISABLED=1`。設定 `OBIT_HOOK_AUDIT=1` 後可執行 `obit hook audit`。Docker 未啟動時先啟動 Docker Desktop 或 Engine，再批准提議。復原目標已存在時 `obit resurrect` 不會覆蓋，而會使用不衝突的名稱；先處理衝突後再試。
