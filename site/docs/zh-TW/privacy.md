# 隱私政策

訃告的帳本、墓地、日誌和設定都保存在你的電腦上。訃告不會上傳當機報告、檔案、路徑或檔案內容。

## 使用統計

使用統計預設開啟，只用來計算使用中的安裝數量及各版本的分布。實際使用時，訃告可能會傳送隨機產生的安裝 ID、版本、作業系統、架構、介面語言和安裝管道，每 24 小時最多一次。訃告不會傳送檔案、路徑或指令，也不會儲存 IP 位址或 User-Agent 標頭。執行 `obit config telemetry off` 或設定 `OBIT_TELEMETRY=0` 即可停止回報。在 CI 環境中，或設有 `DO_NOT_TRACK=1` 時，不會回報。

伺服器不儲存 IP 位址、User-Agent 或原始安裝 ID。以獨立金鑰計算的摘要可用來比對來自同一份安裝的回報，但無法識別個人。每日活動紀錄會在 400 天後到期；為避免重複計數，長期只保留摘要及首次、末次觀測日期。各安裝管道只統計成功回報的總數。只有維護者通過驗證後才能查看儀表板。

關閉回報會清除本機的安裝 ID 和尚未送出的回報。更新檢查有獨立的開關。

## 檢查新版本

- 訃告每天最多讀取一次公開的發行資訊：先查詢 `https://api.github.com` 上訃告儲存庫的最新版本；如果 GitHub 沒有回應，則讀取中國大陸鏡像站的 `https://cn.obituary.woodfish.site/download/latest.json`。
- 請求只包含一般網頁請求都會帶的資訊：你的 IP 位址，以及載明訃告版本的 User-Agent（例如 `Obituary/1.0.0`）。
- 你可以隨時在小視窗的「設定」中關閉「自動檢查更新」，或在 `%APPDATA%\obituary\config.toml` 中加入：

  ```toml
  [update]
  check = false
  ```

  關閉後，只有當你自行執行 `obit update --check` 時，訃告才會檢查更新。

## 安裝和下載

- Scoop、winget 和安裝指令稿會從 [GitHub Releases](https://github.com/woodfishhhh/obituary/releases) 下載壓縮檔，適用 [GitHub 隱私權聲明](https://docs.github.com/site-policy/privacy-policies/github-general-privacy-statement)。
- 使用鏡像站（`-Mirror cn`）時，檔案來自 `cn.obituary.woodfish.site`。鏡像伺服器會保留一般的存取日誌（時間、IP 位址和請求的網址），以便排查問題。
- 視窗中的「立即更新」「下載並重新啟動以更新」，以及 `obit update --apply`（含手動解壓縮的版本），會從 GitHub Releases 下載發行套件和 `checksums.txt`；GitHub 十秒內沒有回應時，改從 `cn.obituary.woodfish.site` 下載同一版本。請求只帶一般請求會帶的內容（IP 位址與標示訃告版本的 User-Agent），不傳送使用資料。
- 發行新版本時，GitHub Actions 會將檔案推送到鏡像站。鏡像站在本機驗證並提供檔案，不會將你的下載請求轉給 GitHub。

## 接入 agent

`obit init` 只會修改你電腦上的 agent 設定檔，並會先讓你查看預計的變更。它不會傳送任何資訊給 agent 供應商或其他地方。

## 官網

- 官網 <https://obituary.woodfish.site/> 不使用流量分析或追蹤指令碼。中國大陸的訪客由我們的伺服器提供服務，其他地區則由 GitHub Pages 提供服務。兩者都會保留一般的存取日誌。
- 你在官網留下的電子郵件地址只用於通知新支援的平台和新版本。地址儲存在我們自己的伺服器上，不會提供給第三方。每封郵件都有取消訂閱的連結。

## 聯絡

隱私相關問題請在 [GitHub 議題](https://github.com/woodfishhhh/obituary/issues)中提出；安全問題請依 [SECURITY.md](https://github.com/woodfishhhh/obituary/blob/main/SECURITY.md) 的說明私下通報。
