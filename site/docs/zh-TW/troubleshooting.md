# 疑難排解

出了問題，先執行 `obit doctor`。它先給一個簡短的結論：第一行說明一切正常還是發現了幾個問題，下面每個面向（安裝、背景執行、agent 整合、右鍵選單、資料、hook 等）佔一行，標著 `PASS`、`WARN` 或 `FAIL`；哪個面向有問題，就在它下面寫出原因和一條可以直接複製的修復指令。每次執行還會把完整報告存成一個自包含的 HTML 檔，放在資料目錄的 `reports` 資料夾裡，只留在你的電腦上，1 天後自動下葬；最後幾行會告訴你它在哪。`obit doctor --open` 用瀏覽器開啟它；`obit doctor --verbose` 在終端機印出完整文字版：逐項檢查，每一行以 `PASS`、`INFO`、`WARN` 或 `FAIL` 開頭，後面是原因和該怎麼做，再加上每個 agent 具體接上了哪些功能。出現 `FAIL` 時，命令會以非零的結束碼結束。要把結果給別人看，加上 `--json`（格式沒有改變）。本頁下面提到的 `path`、`obitd` 這類行，指的是 `--verbose` 和報告裡的單項檢查。

下面每一條都按同樣的順序寫：你會看到什麼、為什麼、怎麼辦、怎麼確認已經好了。

升級後，若檔案總管整套右鍵選單標籤使用另一種支援的語言，選單仍可正常使用。`obit doctor` 只會顯示資訊提示（JSON 為 `installation: label-language-mismatch`）。執行 `obit init --context-menu --yes` 即可換成目前的介面語言。

## 終端機裡找不到 `obit`

終端機提示 `command not found`，PowerShell 說無法辨識 `obit`，原因是程式在磁碟上，但它所在的資料夾不在 PATH 裡。Windows 的安裝腳本會把 `%LOCALAPPDATA%\Programs\obituary` 加進使用者 PATH，不過已經開啟的終端機讀不到新的 PATH。`install.sh` 把程式放進 `~/.local/bin`，從來不改 PATH，有的系統預設也沒有這個資料夾。Ubuntu 的 `.deb` 把程式放在 `/usr/libexec/obituary/`，同樣不在 PATH 裡。

Windows 上把終端機關掉重開就行。macOS 和 Linux 要把 `~/.local/bin` 加進 shell 設定，例如 zsh 在 `~/.zshrc` 裡加一行 `export PATH="$HOME/.local/bin:$PATH"`，再開一個新的終端機。只裝了 `.deb` 的話，改用完整路徑 `/usr/libexec/obituary/obit`，或另外用安裝腳本裝一份命令列。

`obit --version` 能印出版本號，`obit doctor` 裡的 `path` 那一行不再是 `WARN`，就是好了。

## `obit doctor` 說找不到 `obitd`

`obit doctor` 裡出現 `FAIL obitd`，表示 `obit` 旁邊沒有背景程式 `obitd`。訃告會到同一個資料夾裡找它，所以只複製 `obit` 一個檔案是不夠的。把 `obit` 和 `obitd` 放在一起，或依照你的安裝方式重新安裝；之後再執行 `obit doctor`，這一行變成 `PASS` 就是好了。

## 下載的檔案核對不過

用安裝腳本的話，這一步由腳本自己做，雜湊對不上它就會停下，什麼都不會裝。只有手動下載壓縮檔、DMG 或 `.deb` 時，才需要自己核對。對不上的原因通常是下載不完整、檔案在途中被改動，或是 `checksums.txt` 取自另一個版本，因為雜湊是依版本計算的。

從同一個發行頁重新下載安裝檔和 `checksums.txt`（在中國大陸可以從鏡像下載），再核對一次，仍然對不上就不要執行。把下面的 `VERSION` 和檔名換成你下載的那一個。Linux：

```bash
file=obituary_VERSION_linux_amd64.deb
grep -F "  $file" checksums.txt | sha256sum --check --strict -
```

macOS 把最後一行裡的 `sha256sum --check --strict -` 換成 `shasum -a 256 -c -`。Windows PowerShell：

```powershell
$file = "obituary_VERSION_windows_amd64.zip"
(Get-FileHash $file -Algorithm SHA256).Hash -eq ((Select-String -Path checksums.txt -SimpleMatch $file).Line -split '\s+')[0]
```

Linux 和 macOS 印出檔名加 `OK`，PowerShell 印出 `True`，就表示對上了。

## Windows 跳出「Windows 已保護您的電腦」

用瀏覽器手動下載的壓縮檔，Windows SmartScreen 可能會攔一下，原因是訃告的程式還沒有程式碼簽章。用 Scoop、winget 或安裝腳本安裝，一般遇不到。先依上一條核對 SHA-256，對上了再按「其他資訊」，然後按「仍要執行」。核對不了，或系統回報的是惡意軟體，而不只是「未知的發行者」，就不要執行。

## Windows 的小視窗打不開

從開始功能表或系統匣點「訃告」沒有反應，或是提示缺少執行階段，是因為視窗靠 Microsoft Edge WebView2 執行階段繪製。Windows 11 內建它，Windows 10 多數也已經裝好，沒有的機器視窗就起不來。到 Microsoft 官網下載並安裝 WebView2 執行階段（選常青版），再點一次「訃告」，視窗開啟就是好了。

視窗目前只有 x64 版，ARM64 的 Windows 壓縮檔裡沒有 `obit-gui.exe`，只能用命令列。系統匣選單裡的「開啟訃告」跳出通知、說找不到視窗程式，是因為 `obit-gui.exe` 不在 `obitd.exe` 旁邊，ARM64 壓縮檔就是這樣；x64 的把它們放進同一個資料夾即可。

## macOS：app 提示與目前系統版本不相容

1.6.0 的 macOS 小視窗（DMG 裡的 `Obituary.app`）需要 macOS 26。在 macOS 15 及更早的系統上，開啟它時系統會提示這個 app 與目前的版本不相容，無法開啟。這是 1.6.0 打包時留下的問題，1.6.1 修正。在那之前，要使用這個視窗就得升級到 macOS 26。問題只出在 DMG 裡的小視窗上。裝上 1.6.1 或更新的版本，app 能正常開啟，就是好了。

## macOS：「無法驗證開發者」

DMG 裡的 app 沒有 Developer ID 簽名，也沒有公證，所以第一次開啟會被 Gatekeeper 擋下。核對 SHA-256 之後，先正常開啟一次；系統說無法驗證開發者，就到「系統設定 → 隱私權與安全性」，在頁面下方找到 Obituary，按「仍要打開」，再確認一次。詳細步驟在[第一次開啟未簽名的 app](./desktop#第一次開啟未簽名的-app)。不要為此全域關閉 Gatekeeper。系統如果說 app 已損壞或含有惡意軟體，那是另一回事，不要放行。

## 系統匣沒有隨登入啟動（macOS、Ubuntu）

登入之後選單列或系統匣裡沒有訃告的圖示。隨登入啟動靠一個自動啟動項目：macOS 上是使用者 LaunchAgent `site.woodfish.obituary.tray.supervised`，Ubuntu 上是 `obituary-tray.service`。小視窗設定裡的開關沒開，或是 app 搬過位置、是在 DMG 裡直接執行時開啟的，項目裡記的路徑就失效了，登入時找不到程式。

開啟小視窗，在 Settings 裡把「Start the tray at sign-in」關掉再打開，讓訃告重寫這個項目。macOS 上執行 `launchctl list | grep obituary`，能看到 `site.woodfish.obituary.tray.supervised`；Ubuntu 上 `systemctl --user status obituary-tray.service` 可查看服務狀態。

## Ubuntu：小視窗打不開，或沒有系統匣圖示

從應用程式清單點「訃告」沒有反應，一般是缺少 GTK 3 或 WebKitGTK 4.1。用 `apt install ./….deb` 安裝時它們會自動裝上，只有後來被移除，或安裝到一半出錯才會缺。Ubuntu 24.04 裡 GTK 3 的套件叫 `libgtk-3-0t64`，名稱不帶 `t64` 的 `libgtk-3-0` 在這個版本裡不存在，照著舊教學輸入會失敗：

```bash
sudo apt update
sudo apt install libgtk-3-0t64 libwebkit2gtk-4.1-0 xdg-utils
```

裝完再從應用程式清單開啟，視窗出現就是好了。視窗能開、只是沒有系統匣圖示，是因為系統匣要靠桌面提供 StatusNotifierItem，GNOME 預設沒有。不裝擴充功能也能從應用程式清單開啟同一個視窗；想要圖示，就替 GNOME 裝一個支援它的擴充功能，例如 AppIndicator。

## 到期的項目沒有被下葬

執行 `obit daemon status` 查看排程工作。若要立即處理已到期的項目，請執行 `obit reap`。

桌面版啟用監督後，由常駐托盤在到期時處理，另有獨立工作每小時檢查一次。資料目錄未啟用系統匣監督時，每 5 分鐘檢查一次。

`obit doctor` 會列出托盤的監督方式、近期失敗次數及上次結束原因，並區分等待重啟、暫停、主動停止、監督項目遺失或監督程序已結束。

執行 `obit daemon enable` 裝上排程工作（在 Windows 上同時會裝登入時啟動的系統匣）；眼前就要清理，執行 `obit sweep`。Linux 上若沒有 systemd 使用者服務（有些容器和精簡系統是這樣），`obit daemon enable` 裝不上，就改用 cron，每 5 分鐘執行一次 `obitd --once --home <資料目錄> --config-dir <設定目錄>`，這兩個目錄 `obit config` 都會告訴你。`obit daemon status` 裡 `runner` 不再是 `off`，就是好了；Linux 上還可以用 `systemctl --user list-timers | grep obituary` 看到它。

## 到期了，但沒有下葬成功

小視窗的「For you」頁（待你決定）裡，「Burials that failed」（沒埋成的）下有這一項，旁邊寫著原因，常見的是檔案正被編輯器、同步軟體或別的程式占用，檔案還留在原處。下葬要把檔案搬走，被占用或沒有權限時搬不動。訃告不會硬來，原檔案保持原樣，過一陣子再試，間隔越來越長，多次失敗後停下，等你決定。

先關掉占用檔案的程式，再按「Retry」，或是執行 `obit sweep`。不想再管它，就按「Stop tracking」，檔案留在原處，不再倒數。這一項從「Burials that failed」裡消失，並出現在 `obit graveyard` 裡，就是下葬成功了。

## 復活之後檔名變了

`obit resurrect` 沒有報錯，放回來的檔案卻叫 `report (resurrected).txt`，是因為原位置已經有同名的東西了。復活從不覆蓋，名稱被占用時，就在同一個資料夾裡換個名稱放回來，兩份內容都完好。自己比較一下，留下要的那份，把名稱改回去。

## 設定檔寫錯了

`obit config` 回報 `config file has errors`，後面跟著檔案路徑、行號和鍵名，是因為 `config.toml` 裡有訃告不認得的鍵，或是值不合法。訃告不會默默忽略它們，否則 `retention = 30` 會讓 7 天的保留期照舊生效，你卻以為已經改了。依提示裡的行號修正拼字、引號或時間長度，再執行 `obit config`；不再報錯，`obit doctor` 裡的 `config` 那一行是 `PASS`，就是好了。各項的寫法見[設定參考](./config)。

## hook 沒有生效

`obit init` 裝完，agent 刪檔案時沒有被攔下，或是什麼提示都沒出現。常見的原因有三個：agent 沒有重新啟動，還在使用舊設定；這個 agent 的整合裝在另一個範圍（全域還是專案）；agent 的環境裡設了 `OBIT_DISABLED=1`（關閉所有 hook）或 `OBIT_GUARD_RM=0`（關閉刪除護欄）。

先看裝了什麼：

```bash
obit init -g --show        # 全域裝了什麼
obit init --show           # 目前專案裝了什麼
obit doctor --verbose      # 每個 agent 的接入情況
```

沒有裝，就用 `obit init -g --agent <名稱>` 裝上，想先看計畫就加上 `--dry-run`。裝完重新啟動 agent；Codex 還要在裡面執行 `/hooks`，信任訃告的 hook。想知道 hook 到底收到了什麼，在啟動 agent 的終端機裡先設定 `OBIT_HOOK_AUDIT=1`，用一陣子之後執行 `obit hook audit`。被攔下的刪除命令不需要這個開關，本來就會記錄。hook 自己的錯誤寫在資料目錄的 `hook.log` 裡，`obit doctor` 的 `hook-log` 那一行會統計最近 24 小時的錯誤。

`obit doctor` 裡這個 agent 的那一行（例如 `claude`）是 `PASS`、寫著 `integration is current`，就是裝好了；使用刪除護欄的話，`claude-guard-rm` 這樣的行要寫 `registered: yes`。在啟動 agent 的同一個終端機裡執行 `obit doctor`，如果環境變數把 hook 關掉了，這一行會變成 `WARN`，並寫出是哪個變數。

## Docker 容器的提議無法批准

小視窗裡容器提議的「Approve」點不了，卡片上寫著「Docker is not running: start Docker to decide on this proposal.」，意思是 Docker 沒有執行，要先啟動它才能處理這則提議。登記容器要向 Docker 確認它完整的 ID 和建立時間，Docker 沒執行就確認不了。啟動 Docker Desktop 或 Docker Engine，再批准；「Approve」可以點了，`obit docker list` 裡列出這個容器，就是好了。

容器到期時，訃告只是把它停掉，可寫層保留，復活時再重新啟動；保留期過後才會刪除已停止的容器。

## 無法確認 Docker 資料卷已下葬

訃告在刪除前後核對 Docker 事件。若無法確認已刪除，請保留驗證過的封存檔並查看 `obit doctor`，不要視為下葬成功。 每條 `docker-volume-recovery` 警告對應一筆待核實的記錄。請依提示在原 Docker 常駐程式上檢查卷；未提交的封存檔不能用 `obit resurrect` 或 `obit purge` 處理。 [Docker 容器與資料卷](./docker)

## 更新失敗，或在中國大陸下載很慢

`obit update --check` 回報錯誤，或安裝腳本下載很慢、失敗，多半是這台電腦連不上 GitHub。`obit update --check` 和安裝腳本在 GitHub 遲遲沒有回應時，會自己改走中國大陸的鏡像。想一開始就走鏡像，macOS 和 Linux 執行 `bash install.sh --mirror cn`，Windows 執行 `.\install.ps1 -Mirror cn`。手動下載的話，壓縮檔和 `checksums.txt` 要取自同一個版本、同一個來源。`obit update --check` 能印出 `Latest:` 那一行，就是好了。

## 到期的檔案仍在原處

請關閉開啟檔案的程式後重試。若只是行程資訊不完整，可在 `propose` 模式逐項核准。 自動模式必須能檢查檔案擁有者的所有行程。只有檔案屬於目前使用者，且群組及其他人均無寫入權限時，才會忽略其他使用者（包括 root）無法讀取的行程。選擇 `propose` 可逐項核准。核准可以接受行程資訊不完整，但只要確認檔案仍開啟，就會保留檔案並顯示原因。 `obit doctor --verbose` · `obit watch --list`
