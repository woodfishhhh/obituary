#Requires -Version 5.1
<#
.SYNOPSIS
Installs a specific Obituary release after mandatory SHA-256 verification.
.EXAMPLE
./install.ps1 -Version 1.2.3 -WhatIf
.EXAMPLE
./install.ps1 -Version 1.2.3 -InstallDir C:\Temp\obituary -NoPath
.NOTES
The installer downloads the archive, verifies its checksum, then extracts it.
Checksum verification is mandatory; there is no bypass switch.
Close Obituary programs before upgrading. Use -Uninstall to remove this
installation and its integrations while retaining the ledger and graveyard.
BaseUrl may point to an HTTPS mirror or a loopback HTTP snapshot server.
#>
[CmdletBinding(SupportsShouldProcess = $true, DefaultParameterSetName = 'Install')]
param(
    [Parameter(Mandatory = $true, ParameterSetName = 'Install')]
    [ValidatePattern('^v?[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.+-]+)?$')]
    [string]$Version,

    [ValidateSet('amd64', 'arm64')]
    [string]$Architecture,

    [ValidateNotNullOrEmpty()]
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'Programs\obituary'),

    [uri]$BaseUrl,

    [switch]$NoPath,

    [Parameter(Mandatory = $true, ParameterSetName = 'Uninstall')]
    [switch]$Uninstall,

    # Explicit destination also lets installer tests stay out of the real menu.
    [ValidateNotNullOrEmpty()]
    [string]$StartMenuDir = [Environment]::GetFolderPath('Programs')
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
    throw 'Obituary currently supports Windows only.'
}
if (-not $Architecture) {
    $nativeArchitecture = $env:PROCESSOR_ARCHITEW6432
    if (-not $nativeArchitecture) { $nativeArchitecture = $env:PROCESSOR_ARCHITECTURE }
    switch ($nativeArchitecture) {
        'AMD64' { $Architecture = 'amd64' }
        'ARM64' { $Architecture = 'arm64' }
        default { throw "Unsupported Windows architecture: $nativeArchitecture" }
    }
}
$Architecture = $Architecture.ToLowerInvariant()
if (-not $Uninstall) { $Version = $Version -replace '^v', '' }
if (-not $BaseUrl -and -not $Uninstall) {
    $BaseUrl = [uri]"https://github.com/woodfishhhh/obituary/releases/download/v$Version/"
}
if (-not $Uninstall -and (-not $BaseUrl.IsAbsoluteUri -or
    ($BaseUrl.Scheme -ne 'https' -and -not ($BaseUrl.Scheme -eq 'http' -and $BaseUrl.IsLoopback)) -or
    $BaseUrl.Query -or $BaseUrl.Fragment -or $BaseUrl.UserInfo)) {
    throw 'BaseUrl must be an HTTPS directory URL (or loopback HTTP for local snapshots).'
}
$InstallDir = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($InstallDir)
$StartMenuDir = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($StartMenuDir)
# ASCII source remains readable by Windows PowerShell 5.1 without a UTF-8 BOM.
$shortcutName = ([string][char]0x8ba3) + ([char]0x544a) + ' Obituary.lnk'
$shortcutPath = Join-Path $StartMenuDir $shortcutName
$receiptPath = Join-Path $InstallDir '.obituary-install.json'
$archiveName = "obituary_${Version}_windows_${Architecture}.zip"
$source = if ($BaseUrl) { $BaseUrl.AbsoluteUri.TrimEnd('/') } else { '' }
$action = if ($Uninstall) { 'Run obit uninstall --yes (keep data), remove owned shortcut and installed files' } else {
    "Download $archiveName and checksums.txt from $source, verify SHA-256, install and create Start menu shortcut"
}
if (-not $NoPath) { $action += $(if ($Uninstall) { ', then remove the unchanged installer-owned PATH entry' } else { ', then add the directory to the user PATH' }) }
if (-not $PSCmdlet.ShouldProcess($InstallDir, $action)) { return }

# Windows PowerShell 5.1's .NET APIs do not expose link counts or no-follow handles.
# Use Win32 for those checks/pins and handle deletion; System.IO.File.Move performs same-volume
# entry renames without rewriting an existing file (unlike Copy-Item -Force).
Add-Type -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.IO;
using System.Runtime.InteropServices;
using Microsoft.Win32.SafeHandles;
public static class ObituaryInstall {
    [StructLayout(LayoutKind.Sequential)]
    struct Info {
        public uint Attributes;
        public System.Runtime.InteropServices.ComTypes.FILETIME Creation, Access, Write;
        public uint Volume, SizeHigh, SizeLow, Links, IndexHigh, IndexLow;
    }
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    static extern SafeFileHandle CreateFileW(string path, uint access, uint share,
        IntPtr security, uint disposition, uint flags, IntPtr template);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool GetFileInformationByHandle(SafeFileHandle handle, out Info info);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool SetFileInformationByHandle(SafeFileHandle handle, int kind, ref byte data, uint size);

    static void Inspect(SafeFileHandle handle, string path, bool directory) {
        Info info;
        if (!GetFileInformationByHandle(handle, out info))
            throw new Win32Exception(Marshal.GetLastWin32Error(), "Cannot inspect " + path);
        if ((info.Attributes & 0x400) != 0)
            throw new IOException("Refusing reparse point: " + path);
        if (((info.Attributes & 0x10) != 0) != directory)
            throw new IOException("Refusing unexpected file or directory: " + path);
        if (!directory && info.Links > 1)
            throw new IOException("Refusing file with multiple hard links: " + path);
    }

    public static SafeFileHandle Check(string path, bool directory) {
        // OPEN_REPARSE_POINT | BACKUP_SEMANTICS; directories deny write/delete
        // sharing so the checked chain cannot be renamed or turned into a link.
        var handle = CreateFileW(path, 0x80, directory ? 1u : 7u,
            IntPtr.Zero, 3, 0x02200000, IntPtr.Zero);
        int error = Marshal.GetLastWin32Error();
        try {
            if (handle.IsInvalid) {
                if (!directory && error == 2) { handle.Dispose(); return null; }
                throw new Win32Exception(error, "Cannot inspect " + path);
            }
            Inspect(handle, path, directory);
            return handle;
        } catch { handle.Dispose(); throw; }
    }

    // .NET Framework File.Delete reopens a name. Keep one no-write/no-delete
    // sharing handle from inspection through hashing, execution and deletion.
    // FILE_DISPOSITION_INFO is the documented Win32 handle deletion contract.
    public static FileStream PinFile(string path) {
        var handle = CreateFileW(path, 0x80010000, 1, IntPtr.Zero, 3, 0x02200000, IntPtr.Zero);
        int error = Marshal.GetLastWin32Error();
        try {
            if (handle.IsInvalid) {
                if (error == 2) { handle.Dispose(); return null; }
                throw new Win32Exception(error, "Cannot pin " + path);
            }
            Inspect(handle, path, false);
            return new FileStream(handle, FileAccess.Read);
        } catch { handle.Dispose(); throw; }
    }

    public static string Hash(FileStream stream) {
        stream.Position = 0;
        using (var hash = System.Security.Cryptography.SHA256.Create())
            return BitConverter.ToString(hash.ComputeHash(stream)).Replace("-", "");
    }

    public static string Text(FileStream stream) {
        stream.Position = 0;
        using (var reader = new StreamReader(stream, System.Text.Encoding.UTF8, true, 1024, true))
            return reader.ReadToEnd();
    }

    static void Disposition(FileStream stream, bool delete) {
        byte disposition = delete ? (byte)1 : (byte)0;
        if (!SetFileInformationByHandle(stream.SafeFileHandle, 4, ref disposition, 1))
            throw new Win32Exception(Marshal.GetLastWin32Error(), "Cannot delete pinned file");
    }

    public static void Delete(FileStream stream) {
        Disposition(stream, true);
        stream.Dispose();
    }

    public static void Complete(FileStream cli, FileStream receipt) {
        // Validate receipt deletion before retiring the recovery executable.
        // If the last payload refuses deletion, cancel the pending receipt
        // deletion while its same handle is still open, so retry is possible.
        Disposition(receipt, true);
        try { Disposition(cli, true); }
        catch { Disposition(receipt, false); throw; }
        cli.Dispose();
        receipt.Dispose();
    }

    public static List<SafeFileHandle> PinDirectories(string path) {
        var chain = new Stack<string>();
        for (var dir = new DirectoryInfo(path); dir != null; dir = dir.Parent)
            chain.Push(dir.FullName);
        var handles = new List<SafeFileHandle>();
        try {
            foreach (string dir in chain) {
                Directory.CreateDirectory(dir);
                handles.Add(Check(dir, true));
            }
            return handles;
        } catch {
            foreach (var handle in handles) handle.Dispose();
            throw;
        }
    }
}
'@

# WScript.Shell creates the Start Menu shortcut through the standard Windows API.
function Update-StartMenuShortcut([bool]$Remove) {
    if ($Remove) { Remove-StartMenuShortcut; return }
    $pins = [ObituaryInstall]::PinDirectories($StartMenuDir)
    # WScript.Shell cannot load or save a .lnk whose name has characters
    # outside the system ANSI code page (the Chinese name on English
    # Windows). It works on an ASCII-named copy in the same folder, which is
    # then copied into place under the real name.
    $work = Join-Path $StartMenuDir ('obituary-shortcut-' + [guid]::NewGuid().ToString('N') + '.lnk')
    try {
        $handle = [ObituaryInstall]::Check($shortcutPath, $false)
        if ($handle) { $handle.Dispose() }
        $shell = New-Object -ComObject WScript.Shell
        $target = Join-Path $InstallDir 'obit-gui.exe'
        if (Test-Path -LiteralPath $shortcutPath) {
            [IO.File]::Copy($shortcutPath, $work)
            $existing = $shell.CreateShortcut($work)
            if ($existing.TargetPath -ine $target -or $existing.Description -ne 'Obituary installer shortcut') {
                throw "Preserving a shortcut not owned by this installation: $shortcutPath"
            }
            [IO.File]::Delete($work)
        }
        $shortcut = $shell.CreateShortcut($work)
        $shortcut.TargetPath = $target
        $shortcut.WorkingDirectory = $InstallDir
        $shortcut.IconLocation = "$target,0"
        $shortcut.Description = 'Obituary installer shortcut'
        $shortcut.Save()
        [IO.File]::Copy($work, $shortcutPath, $true)
        Write-Output "Start menu shortcut: $shortcutPath"
    }
    finally {
        if (Test-Path -LiteralPath $work) { [IO.File]::Delete($work) }
        foreach ($handle in $pins) { $handle.Dispose() }
    }
}

function Remove-StartMenuShortcut {
    if (-not (Test-Path -LiteralPath $shortcutPath)) { return }
    $pins = [ObituaryInstall]::PinDirectories($StartMenuDir)
    $file = $null
    $snapshot = $null
    try {
        $file = [ObituaryInstall]::PinFile($shortcutPath)
        if (-not $file) { return }
        # WScript may open without delete sharing. Parse a private copy of the
        # pinned bytes while the original object remains locked for deletion.
        $snapshot = Join-Path ([IO.Path]::GetTempPath()) ('obituary-shortcut-' + [guid]::NewGuid().ToString('N') + '.lnk')
        $copy = [IO.File]::Open($snapshot, 'CreateNew', 'Write', 'None')
        try { $file.CopyTo($copy) } finally { $copy.Dispose() }
        $shell = New-Object -ComObject WScript.Shell
        $shortcut = $shell.CreateShortcut($snapshot)
        if ($shortcut.TargetPath -ine (Join-Path $InstallDir 'obit-gui.exe') -or
            $shortcut.Description -ne 'Obituary installer shortcut') {
            throw "Preserving a shortcut not owned by this installation: $shortcutPath"
        }
        [ObituaryInstall]::Delete($file)
    }
    finally {
        if ($file) { $file.Dispose() }
        if ($snapshot) { [IO.File]::Delete($snapshot) }
        foreach ($handle in $pins) { $handle.Dispose() }
    }
}

function Read-InstallReceipt([IO.FileStream]$Stream) {
    if (-not $Stream) { throw "No installer receipt at $receiptPath; reinstall first to establish file ownership." }
    $receipt = [ObituaryInstall]::Text($Stream) | ConvertFrom-Json
    if ($receipt.owner -ne 'obituary-install-v1' -or $receipt.directory -ine $InstallDir) {
        throw "Invalid installation ownership receipt: $receiptPath"
    }
    $allowed = @('obit.exe', 'obitd.exe', 'obit-gui.exe', 'LICENSE', 'README.txt')
    foreach ($file in $receipt.files) {
        if ($allowed -cnotcontains $file.name -or $file.sha256 -cnotmatch '^[0-9a-fA-F]{64}$') {
            throw "Invalid installed-file receipt: $receiptPath"
        }
    }
    return $receipt
}

function Remove-InstallPath($Receipt, $OwnedNames) {
    # Old receipts cannot prove who added PATH. Changed spelling, duplicates,
    # and other programs left in the directory also require preserving it.
    if ($NoPath -or $Receipt.PSObject.Properties.Name -notcontains 'pathAdded' -or
        $Receipt.pathAdded -isnot [bool] -or -not $Receipt.pathAdded) { return }
    if ($Receipt.PSObject.Properties.Name -notcontains 'pathEntry' -or
        $Receipt.pathEntry -cne $InstallDir) { throw 'Invalid PATH ownership receipt; PATH retained.' }
    $unknown = @(Get-ChildItem -LiteralPath $InstallDir -Force | Where-Object {
        $_.Name -cne '.obituary-install.json' -and $OwnedNames -cnotcontains $_.Name
    })
    if ($unknown.Count -gt 0) { Write-Output 'Preserving PATH for other or modified files.'; return }
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $entries = @($userPath -split ';')
    if (@($entries | Where-Object { $_ -ceq $Receipt.pathEntry }).Count -ne 1) {
        Write-Output 'Preserving PATH: the recorded entry changed or is ambiguous.'
        return
    }
    $remaining = @($entries | Where-Object { $_ -cne $Receipt.pathEntry }) -join ';'
    [Environment]::SetEnvironmentVariable('Path', $remaining, 'User')
    $env:PATH = @($env:PATH -split ';' | Where-Object { $_ -cne $Receipt.pathEntry }) -join ';'
}

function Uninstall-ReleaseFiles {
    if (-not (Test-Path -LiteralPath $InstallDir)) {
        Write-Output "Installation directory is absent: $InstallDir"
        return
    }
    $pins = [ObituaryInstall]::PinDirectories($InstallDir)
    $opened = [Collections.Generic.List[IO.FileStream]]::new()
    try {
        $receiptFile = [ObituaryInstall]::PinFile($receiptPath)
        if ($receiptFile) { $opened.Add($receiptFile) }
        $receipt = Read-InstallReceipt $receiptFile
        $owned = @()
        foreach ($file in $receipt.files) {
            $path = Join-Path $InstallDir $file.name
            $handle = [ObituaryInstall]::PinFile($path)
            if (-not $handle) { continue }
            $opened.Add($handle)
            if ([ObituaryInstall]::Hash($handle) -ine $file.sha256) {
                Write-Output "Preserving modified installed file: $path"
            } else { $owned += @{ Name = $file.name; Stream = $handle } }
        }
        # Never execute a replaced obit.exe, and never delete binaries when its
        # integration cleanup failed (the remaining executable enables retry).
        if (@($owned | Where-Object { $_.Name -ceq 'obit.exe' }).Count -ne 1) { throw 'Owned obit.exe is unavailable; integrations were not removed.' }
        & (Join-Path $InstallDir 'obit.exe') uninstall --yes
        if ($LASTEXITCODE -ne 0) { throw "obit uninstall failed ($LASTEXITCODE); installation files retained." }
        Update-StartMenuShortcut $true
        Remove-InstallPath $receipt @($owned | ForEach-Object { $_.Name })
        # All files were opened with DELETE access before executing anything.
        # Keep the verified CLI until every other owned payload is removed.
        foreach ($file in @($owned | Where-Object { $_.Name -cne 'obit.exe' })) {
            [ObituaryInstall]::Delete($file.Stream)
        }
        $cli = @($owned | Where-Object { $_.Name -ceq 'obit.exe' })[0]
        [ObituaryInstall]::Complete($cli.Stream, $receiptFile)
    }
    finally {
        foreach ($file in $opened) { $file.Dispose() }
        foreach ($handle in $pins) { $handle.Dispose() }
    }
    if (@(Get-ChildItem -LiteralPath $InstallDir -Force).Count -eq 0) {
        [IO.Directory]::Delete($InstallDir, $false)
        Write-Output "Removed installation directory: $InstallDir"
    } else {
        Write-Output "Preserving directory containing other or modified files: $InstallDir"
    }
}

if ($Uninstall) { Uninstall-ReleaseFiles; return }

function Install-ReleaseFiles($Expanded, $Names) {
    $pins = [ObituaryInstall]::PinDirectories($InstallDir)
    $transaction = $null
    $removeTransaction = $false
    $saved = [Collections.Generic.List[string]]::new()
    $committed = [Collections.Generic.List[string]]::new()
    try {
        foreach ($name in $Names) {
            $handle = [ObituaryInstall]::Check((Join-Path $InstallDir $name), $false)
            if ($handle) { $handle.Dispose() }
        }
        $transaction = Join-Path $InstallDir ('.obituary-update-' + [guid]::NewGuid().ToString('N'))
        $null = New-Item -ItemType Directory -Path $transaction
        $new = Join-Path $transaction 'new'
        $old = Join-Path $transaction 'old'
        $null = New-Item -ItemType Directory -Path $new, $old
        # Prepare the whole version on the destination volume before moving any
        # old entries. Keep stable executable paths used by PATH and Explorer.
        foreach ($name in $Names) {
            [IO.File]::Copy((Join-Path $Expanded $name), (Join-Path $new $name), $false)
        }
        foreach ($name in $Names) {
            $target = Join-Path $InstallDir $name
            $handle = [ObituaryInstall]::Check($target, $false)
            if ($handle) {
                $handle.Dispose()
                [IO.File]::Move($target, (Join-Path $old $name))
                $saved.Add($name)
            }
        }
        foreach ($name in $Names) {
            [IO.File]::Move((Join-Path $new $name), (Join-Path $InstallDir $name))
            $committed.Add($name)
        }
        $removeTransaction = $true
    }
    catch {
        $failure = $_
        try {
            foreach ($name in $committed) {
                [IO.File]::Move((Join-Path $InstallDir $name), (Join-Path $new $name))
            }
            foreach ($name in $saved) {
                [IO.File]::Move((Join-Path $old $name), (Join-Path $InstallDir $name))
            }
            $removeTransaction = $true
        }
        catch {
            throw "Installation failed: $failure. Recovery files retained at ${transaction}: $_"
        }
        throw $failure
    }
    finally {
        try {
            if ($transaction -and $removeTransaction) {
                Remove-Item -LiteralPath $transaction -Recurse -Force
            }
        }
        finally { foreach ($handle in $pins) { $handle.Dispose() } }
    }
}

# Only this newly created directory is ever removed, including on checksum errors.
$workDir = Join-Path ([IO.Path]::GetTempPath()) ('obituary-install-' + [guid]::NewGuid().ToString('N'))
$null = New-Item -ItemType Directory -Path $workDir
try {
    $archive = Join-Path $workDir $archiveName
    $checksums = Join-Path $workDir 'checksums.txt'
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest -UseBasicParsing -Uri "$source/checksums.txt" -OutFile $checksums
    Invoke-WebRequest -UseBasicParsing -Uri "$source/$archiveName" -OutFile $archive

    $pattern = '^(?<Hash>[0-9a-fA-F]{64})\s+\*?' + [regex]::Escape($archiveName) + '$'
    $expected = @(Get-Content -LiteralPath $checksums | ForEach-Object {
        if ($_ -cmatch $pattern) { $Matches.Hash }
    })
    if ($expected.Count -ne 1) {
        throw "Expected exactly one SHA-256 entry for $archiveName in checksums.txt; found $($expected.Count)."
    }
    $actual = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash
    if ($actual -ine $expected[0]) {
        throw "SHA-256 mismatch for $archiveName; refusing to install."
    }
    Write-Output "SHA-256 verified: $archiveName"

    # Built-in extraction rejects paths outside its destination. Nothing from the
    # download is extracted, copied to the install directory, or executed before this point.
    $expanded = Join-Path $workDir 'expanded'
    Expand-Archive -LiteralPath $archive -DestinationPath $expanded
    $files = @('obit.exe', 'obitd.exe', 'LICENSE', 'README.txt')
    if ($Architecture -eq 'amd64') { $files += 'obit-gui.exe' }
    foreach ($name in $files) {
        if (-not (Test-Path -LiteralPath (Join-Path $expanded $name) -PathType Leaf)) {
            throw "Release archive is missing $name; refusing to install."
        }
    }
    $pathAdded = $false
    # Upgrades retain a previous proven addition; a legacy receipt claims none.
    if (Test-Path -LiteralPath $receiptPath) {
        $previousFile = [ObituaryInstall]::PinFile($receiptPath)
        try {
            $previous = Read-InstallReceipt $previousFile
            $pathAdded = $previous.PSObject.Properties.Name -contains 'pathAdded' -and
                $previous.pathAdded -is [bool] -and $previous.pathAdded -and
                $previous.PSObject.Properties.Name -contains 'pathEntry' -and $previous.pathEntry -ceq $InstallDir
        } finally { if ($previousFile) { $previousFile.Dispose() } }
    }
    $receipt = [ordered]@{ owner = 'obituary-install-v1'; directory = $InstallDir; pathAdded = $pathAdded; pathEntry = $InstallDir; files = @(
        foreach ($name in $files) {
            @{ name = $name; sha256 = (Get-FileHash -LiteralPath (Join-Path $expanded $name) -Algorithm SHA256).Hash }
        }
    ) }
    # The receipt participates in the same pinned-directory transaction as the
    # binaries, including rollback; never rewrite it through a released path.
    [IO.File]::WriteAllText((Join-Path $expanded '.obituary-install.json'), ($receipt | ConvertTo-Json -Depth 4), [Text.UTF8Encoding]::new($false))
    Install-ReleaseFiles $expanded ($files + '.obituary-install.json')
    if ($Architecture -eq 'amd64') { Update-StartMenuShortcut $false }

    if (-not $NoPath) {
        $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        $alreadyPresent = @($userPath -split ';' | Where-Object {
            [Environment]::ExpandEnvironmentVariables($_.Trim().Trim('"')).TrimEnd('\') -ieq $InstallDir.TrimEnd('\')
        }).Count -gt 0
        if (-not $alreadyPresent) {
            $updatedPath = if ([string]::IsNullOrEmpty($userPath)) { $InstallDir } else { $userPath + ';' + $InstallDir }
            [Environment]::SetEnvironmentVariable('Path', $updatedPath, 'User')
            # Commit ownership only after the OS write succeeds. An interruption
            # before this receipt leaves an unclaimed PATH entry, never a false claim.
            $receipt.pathAdded = $true
            [IO.File]::WriteAllText((Join-Path $expanded '.obituary-install.json'), ($receipt | ConvertTo-Json -Depth 4), [Text.UTF8Encoding]::new($false))
            Install-ReleaseFiles $expanded @('.obituary-install.json')
        }
        if (($env:PATH -split ';') -inotcontains $InstallDir) { $env:PATH += ";$InstallDir" }
        Write-Output 'User PATH is ready. Open a new terminal to use obit.'
    }
    Write-Output "Installed Obituary $Version ($Architecture) in $InstallDir"
}
finally {
    Remove-Item -LiteralPath $workDir -Recurse -Force
}
