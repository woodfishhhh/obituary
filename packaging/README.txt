讣告 Obituary

给文件和文件夹设定寿命，到期下葬到墓地，保留期内可以复活。

本包适用于文件名标明的 Windows 架构：
- obit.exe：命令行。
- obitd.exe：后台程序，请与 obit.exe 放在同一目录。
- obit-gui.exe：桌面界面，仅包含在 amd64 包中，需要 WebView2 Runtime。

解压前，可用 PowerShell 的 Get-FileHash <压缩包> -Algorithm SHA256
与同一版本的 checksums.txt 核对。本阶段的程序尚未进行代码签名。

解压后把该目录加入 PATH，执行 obit --version 和 obit --help。
常用命令：obit mark debug.log 2h、obit list、obit graveyard、obit resurrect <id>。
默认墓地保留期为 7 天；配置为回收站模式时，须通过系统回收站恢复。

完整说明：https://github.com/woodfishhhh/obituary#readme
许可证：MIT，见同目录 LICENSE。
