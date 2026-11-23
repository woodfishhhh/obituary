$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if ($env:GITHUB_ACTIONS -ne 'true' -or $env:RUNNER_ENVIRONMENT -ne 'github-hosted') {
    throw 'requires a disposable GitHub-hosted Windows runner'
}
if ($env:EXPECTED_VERSION -notmatch '^\d+\.\d+\.\d+$') {
    throw 'EXPECTED_VERSION must be a stable semantic version'
}

$Evidence = Join-Path $env:RUNNER_TEMP 'obit-explorer-acceptance'
$Work = Join-Path $Evidence 'work'
$Archive = Join-Path $Evidence "obituary_$($env:EXPECTED_VERSION)_windows_amd64.zip"
$Checksums = Join-Path $Evidence 'checksums.txt'
New-Item -ItemType Directory -Force -Path $Evidence, $Work | Out-Null
Start-Transcript -Path (Join-Path $Evidence 'acceptance.log')
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -AssemblyName System.Windows.Forms
Add-Type @'
using System;
using System.Runtime.InteropServices;
public static class ObitInput {
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
  [DllImport("user32.dll")] public static extern void mouse_event(uint flags, uint dx, uint dy, uint data, UIntPtr extra);
}
'@

function Save-Screenshot([string]$Name) {
    try {
        Add-Type -AssemblyName System.Drawing
        try { Add-Type @'
using System;
using System.Drawing;
using System.Drawing.Imaging;
using System.Windows.Forms;
using System.Runtime.InteropServices;
public static class ObitScreen {
  [DllImport("user32.dll")] static extern IntPtr GetDesktopWindow();
  [DllImport("user32.dll")] static extern IntPtr GetWindowDC(IntPtr hWnd);
  [DllImport("user32.dll")] static extern int ReleaseDC(IntPtr hWnd, IntPtr hDc);
  [DllImport("gdi32.dll")] static extern IntPtr CreateCompatibleDC(IntPtr hDc);
  [DllImport("gdi32.dll")] static extern IntPtr CreateCompatibleBitmap(IntPtr hDc, int w, int h);
  [DllImport("gdi32.dll")] static extern IntPtr SelectObject(IntPtr hDc, IntPtr hObject);
  [DllImport("gdi32.dll")] static extern bool BitBlt(IntPtr dst, int x, int y, int w, int h, IntPtr src, int sx, int sy, int rop);
  [DllImport("gdi32.dll")] static extern bool DeleteObject(IntPtr hObject);
  [DllImport("gdi32.dll")] static extern bool DeleteDC(IntPtr hDc);
  public static void Capture(string path) {
    var screen = Screen.PrimaryScreen.Bounds;
    var desktop = GetDesktopWindow(); var src = GetWindowDC(desktop);
    var dc = CreateCompatibleDC(src); var bmp = CreateCompatibleBitmap(src, screen.Width, screen.Height);
    var old = SelectObject(dc, bmp); BitBlt(dc, 0, 0, screen.Width, screen.Height, src, 0, 0, 0x00CC0020);
    SelectObject(dc, old); using (var image = Image.FromHbitmap(bmp)) image.Save(path, ImageFormat.Png);
    DeleteObject(bmp); DeleteDC(dc); ReleaseDC(desktop, src);
  }
}
'@ } catch { }
        [ObitScreen]::Capture((Join-Path $Evidence $Name))
    } catch { Write-Warning "screenshot $Name failed: $($_.Exception.Message)" }
}

function Assert-Equal($Expected, $Actual, [string]$Message) {
    if ($Expected -ne $Actual) { throw "$Message`nexpected: $Expected`nactual:   $Actual" }
}

function Invoke-Cli([string]$Exe, [string[]]$CommandArgs) {
    $output = & $Exe @CommandArgs 2>&1
    if ($LASTEXITCODE -ne 0) { throw "CLI failed ($LASTEXITCODE): $($output -join "`n")" }
    return ($output -join "`n")
}

function Assert-MenuResult([string]$Name) {
    $path = Join-Path $env:OBIT_HOME 'context-menu\last-result.txt'
    for ($i = 0; $i -lt 20 -and -not (Test-Path $path); $i++) { Start-Sleep -Milliseconds 250 }
    if (-not (Test-Path $path)) { throw "$Name did not publish a context-menu result" }
    $result = Get-Content -Raw $path
    if ([string]::IsNullOrWhiteSpace($result)) { throw "$Name published an empty context-menu result" }
    Copy-Item $path (Join-Path $Evidence "$Name-result.txt") -Force
}

function Get-MenuLabels([object]$Item) {
    @($Item.Verbs() | ForEach-Object { $_.Name.Trim() } | Where-Object { $_ })
}

function Find-Explorer([string]$Path) {
    $shell = New-Object -ComObject Shell.Application
    Start-Process explorer.exe -ArgumentList "`"$Path`""
    for ($i = 0; $i -lt 30; $i++) {
        foreach ($window in @($shell.Windows())) {
            if ($window.FullName -and $window.Document.Folder.Self.Path -eq $Path) { return $window }
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Explorer did not open $Path"
}

function Get-UiExplorer([object]$Window) {
    $element = [System.Windows.Automation.AutomationElement]::FromHandle([IntPtr]$Window.HWND)
    [ObitInput]::SetForegroundWindow([IntPtr]$Window.HWND) | Out-Null
    return $element
}

function Select-ExplorerItems([object]$Window, [string[]]$Names) {
    $items = @($Window.Document.Folder.Items())
    $found = @()
    foreach ($name in $Names) {
        $item = $items | Where-Object { $_.Name -eq $name } | Select-Object -First 1
        if (-not $item) { throw "Explorer item not found: $name" }
        $found += $item
    }
    return $found
}

function Invoke-ExplorerVerb([object]$Item, [string]$Needle) {
    $verb = @($Item.Verbs()) | Where-Object { $_.Name.Trim() -like "*$Needle*" } | Select-Object -First 1
    if (-not $verb) { throw "Explorer verb not found: $Needle; available: $((Get-MenuLabels $Item) -join ' | ')" }
    $verb.DoIt()
}

function Find-UiItem([object]$Explorer, [string]$Name) {
    $nameCondition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::NameProperty, $Name)
    $typeCondition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
        [System.Windows.Automation.ControlType]::ListItem)
    $condition = New-Object System.Windows.Automation.AndCondition($nameCondition, $typeCondition)
    for ($i = 0; $i -lt 20; $i++) {
        $item = $Explorer.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $condition)
        if ($item -and -not $item.Current.IsOffscreen) { return $item }
        Start-Sleep -Milliseconds 250
    }
    throw "UI Automation item not found: $Name"
}

function Get-VisibleMenu([object]$Explorer) {
    $menuCondition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
        [System.Windows.Automation.ControlType]::Menu)
    $itemCondition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
        [System.Windows.Automation.ControlType]::MenuItem)
    for ($i = 0; $i -lt 30; $i++) {
        $menus = [System.Windows.Automation.AutomationElement]::RootElement.FindAll([System.Windows.Automation.TreeScope]::Descendants, $menuCondition)
        foreach ($menu in $menus) {
            if (-not $menu.Current.IsOffscreen -and $menu.FindAll([System.Windows.Automation.TreeScope]::Children, $itemCondition).Count -gt 0) { return $menu }
        }
        Start-Sleep -Milliseconds 250
    }
    throw 'Explorer context menu did not appear'
}

function Get-MenuItems([object]$Menu) {
    $condition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
        [System.Windows.Automation.ControlType]::MenuItem)
    @($Menu.FindAll([System.Windows.Automation.TreeScope]::Children, $condition) | ForEach-Object { $_.Current.Name })
}

function Open-ExplorerMenu([object]$Explorer, [string[]]$Names) {
    $first = Find-UiItem $Explorer $Names[0]
    $selection = $first.GetCurrentPattern([System.Windows.Automation.SelectionItemPattern]::Pattern)
    $selection.Select()
    for ($i = 1; $i -lt $Names.Count; $i++) {
        $item = Find-UiItem $Explorer $Names[$i]
        $item.GetCurrentPattern([System.Windows.Automation.SelectionItemPattern]::Pattern).AddToSelection()
    }
    [System.Windows.Forms.SendKeys]::SendWait('+{F10}')
    return Get-VisibleMenu $Explorer
}

function Invoke-MenuItem([object]$Menu, [string]$Needle, [switch]$ExpandOnly) {
    $condition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
        [System.Windows.Automation.ControlType]::MenuItem)
    $item = @($Menu.FindAll([System.Windows.Automation.TreeScope]::Children, $condition) |
        Where-Object { $_.Current.Name -like "*$Needle*" } | Select-Object -First 1)
    if (-not $item) {
        $item = @([System.Windows.Automation.AutomationElement]::RootElement.FindAll(
            [System.Windows.Automation.TreeScope]::Descendants, $condition) |
            Where-Object { -not $_.Current.IsOffscreen -and $_.Current.Name -like "*$Needle*" } |
            Select-Object -First 1)
    }
    if (-not $item) { throw "UI menu item not found: $Needle; available: $((Get-MenuItems $Menu) -join ' | ')" }
    if ($ExpandOnly) {
        $item.GetCurrentPattern([System.Windows.Automation.ExpandCollapsePattern]::Pattern).Expand()
        return
    }
    $item.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
}

function Rename-ExplorerItem([object]$Explorer, [string]$OldName, [string]$NewName) {
    $item = Find-UiItem $Explorer $OldName
    $item.GetCurrentPattern([System.Windows.Automation.SelectionItemPattern]::Pattern).Select()
    [System.Windows.Forms.SendKeys]::SendWait('{F2}')
    Start-Sleep -Milliseconds 300
    [System.Windows.Forms.SendKeys]::SendWait($NewName)
    [System.Windows.Forms.SendKeys]::SendWait('{ENTER}')
    Start-Sleep -Seconds 1
    if (-not (Test-Path (Join-Path $sample $NewName))) { throw "Explorer F2 rename failed: $OldName -> $NewName" }
}

function Invoke-ExplorerAction([object]$Explorer, [string[]]$Names, [string]$Needle, [string]$Screenshot) {
    $menu = Open-ExplorerMenu $Explorer $Names
    $labels = Get-MenuItems $menu
    $labels | ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $Evidence $Screenshot)
    Save-Screenshot ($Screenshot -replace '\.json$', '.png')
    if ($Needle -eq 'Add to Obituary' -or $Needle -eq '放进讣告') {
        Invoke-MenuItem $menu $Needle -ExpandOnly
    } else {
        Invoke-MenuItem $menu $Needle
    }
    Start-Sleep -Seconds 2
}

$base = "https://github.com/woodfishhhh/obituary/releases/download/v$($env:EXPECTED_VERSION)"
Invoke-WebRequest "$base/checksums.txt" -OutFile $Checksums
Invoke-WebRequest "$base/obituary_$($env:EXPECTED_VERSION)_windows_amd64.zip" -OutFile $Archive
$line = Get-Content $Checksums | Where-Object { $_ -match "\s+obituary_$([regex]::Escape($env:EXPECTED_VERSION))_windows_amd64\.zip$" }
if (-not $line) { throw 'checksums.txt has no Windows amd64 archive entry' }
$expectedHash = ($line -split '\s+', 2)[0].ToLowerInvariant()
$actualHash = (Get-FileHash $Archive -Algorithm SHA256).Hash.ToLowerInvariant()
Assert-Equal $expectedHash $actualHash 'release archive SHA-256 mismatch'
Expand-Archive $Archive -DestinationPath $Work -Force
$Obit = Join-Path $Work 'obit.exe'
$Obitd = Join-Path $Work 'obitd.exe'
if (-not (Test-Path $Obit) -or -not (Test-Path $Obitd)) { throw 'release archive lacks obit.exe or obitd.exe' }

$env:OBIT_HOME = Join-Path $Work 'data'
$env:OBIT_CONFIG_DIR = Join-Path $Work 'config'
$env:OBIT_USER_HOME = Join-Path $Work 'user'
$env:OBIT_LANG = ''
New-Item -ItemType Directory -Force -Path $env:OBIT_HOME, $env:OBIT_CONFIG_DIR, $env:OBIT_USER_HOME | Out-Null
$sample = Join-Path $Work 'samples'
New-Item -ItemType Directory -Force -Path $sample, (Join-Path $sample 'folder') | Out-Null
'alpha' | Set-Content (Join-Path $sample 'alpha.txt')
'beta' | Set-Content (Join-Path $sample 'beta.txt')
'folder' | Set-Content (Join-Path $sample 'folder\inside.txt')
"[LocalizedFileNames]`r`nforeign.txt=Keep this foreign label`r`n" | Set-Content -Encoding utf8 (Join-Path $sample 'desktop.ini')
'foreign' | Set-Content (Join-Path $sample 'foreign.txt')

function Set-Language([string]$Language) {
    Set-Content -Encoding utf8 (Join-Path $env:OBIT_CONFIG_DIR 'config.toml') "[ui]`nlanguage = '$Language'`n[explorer]`nenabled = true`neverywhere = true`n"
    $output = Invoke-Cli $Obit @('init', '--yes', '--no-shortcuts', '--no-skill')
    $output | Set-Content -Encoding utf8 (Join-Path $Evidence "init-$Language.txt")
}

function Registry-Labels([string]$Language) {
    $roots = @('Software\Classes\Obituary.ContextMenu', 'Software\Classes\*\shell\Obituary.Mark', 'Software\Classes\Directory\shell\Obituary.Mark', 'Software\Classes\Directory\Background\shell\Obituary.Policy')
    $rows = @(foreach ($root in $roots) {
        $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey($root)
        if ($key) {
            try {
                $shellKey = $key.OpenSubKey('shell')
                try {
                    $children = if ($shellKey) {
                        @($shellKey.GetSubKeyNames() | Sort-Object | ForEach-Object {
                            $child = $shellKey.OpenSubKey($_)
                            try { $child.GetValue('MUIVerb') } finally { $child.Dispose() }
                        })
                    } else { @() }
                } finally { if ($shellKey) { $shellKey.Dispose() } }
                [pscustomobject]@{ Path = "HKCU:\$root"; Label = $key.GetValue('MUIVerb'); Children = $children }
            } finally { $key.Dispose() }
        }
    })
    $rows | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 (Join-Path $Evidence "registry-$Language.json")
    if ($rows.Count -eq 0) { throw "$Language context-menu registry roots were not installed" }
    $children = @($rows[0].Children)
    Assert-Equal 3 $children.Count "$Language duration submenu count"
    Assert-Equal $children[0] ($(if ($Language -eq 'zh-CN') { '1 天' } else { '1 day' })) "$Language first duration label"
    Assert-Equal $children[1] ($(if ($Language -eq 'zh-CN') { '7 天' } else { '7 days' })) "$Language second duration label"
    Assert-Equal $children[2] ($(if ($Language -eq 'zh-CN') { '30 天' } else { '30 days' })) "$Language third duration label"
}

foreach ($language in @('zh-CN', 'en')) {
    Set-Language $language
    Registry-Labels $language
    $shell = New-Object -ComObject Shell.Application
    $ns = $shell.Namespace($sample)
    $file = $ns.ParseName('alpha.txt')
    $folder = $ns.ParseName('folder')
    $fileLabels = Get-MenuLabels $file
    $folderLabels = Get-MenuLabels $folder
    [pscustomobject]@{ Language = $language; FileVerbs = $fileLabels; FolderVerbs = $folderLabels } | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 (Join-Path $Evidence "verbs-$language.json")
    $window = Find-Explorer $sample
    $ui = Get-UiExplorer $window
    $need = if ($language -eq 'zh-CN') { '放进讣告' } else { 'Add to Obituary' }
    $duration = if ($language -eq 'zh-CN') { '7 天' } else { '7 days' }
    Invoke-ExplorerAction $ui @('alpha.txt') $need "single-$language.json"
    $menu = Get-VisibleMenu $ui
    Invoke-MenuItem $menu $duration
    Start-Sleep -Seconds 2
    Assert-MenuResult "mark-$language"
    $listed = Invoke-Cli $Obit @('list')
    if ($listed -notmatch [regex]::Escape((Join-Path $sample 'alpha.txt'))) { throw "$language CLI list did not show the real alpha path" }
    $window.Quit()
    Start-Sleep -Seconds 1
    $window = Find-Explorer $sample
    $ui = Get-UiExplorer $window
    $folderMenu = Open-ExplorerMenu $ui @('folder')
    $folderMenuLabels = Get-MenuItems $folderMenu
    $folderMenuLabels | ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $Evidence "folder-$language.json")
    if (-not (($folderMenuLabels -join "`n") -match [regex]::Escape($need))) { throw "$language folder menu lacks $need" }
    [System.Windows.Forms.SendKeys]::SendWait('{ESC}')
    $window.Quit()
    Start-Sleep -Seconds 1
}

# The last language is English; launch Explorer once for multi-select and background evidence.
$window = Find-Explorer $sample
$ui = Get-UiExplorer $window
$menu = Open-ExplorerMenu $ui @('alpha.txt', 'folder')
$multiLabels = Get-MenuItems $menu
$multiLabels | ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $Evidence 'multi-item-verbs.json')
if (-not (($multiLabels -join "`n") -match 'Add to Obituary')) { throw 'mixed file/folder selection lacks Add to Obituary' }
Save-Screenshot 'multi-selection.png'
$window.Quit()
Start-Sleep -Seconds 1

# Background menus are opened by right-clicking empty view space and must expose policy only.
$window = Find-Explorer $sample
$ui = Get-UiExplorer $window
$rect = $ui.Current.BoundingRectangle
[ObitInput]::SetCursorPos([int]($rect.X + $rect.Width - 80), [int]($rect.Y + $rect.Height - 120)) | Out-Null
[ObitInput]::mouse_event(0x0008, 0, 0, 0, [UIntPtr]::Zero)
[ObitInput]::mouse_event(0x0010, 0, 0, 0, [UIntPtr]::Zero)
$background = Get-VisibleMenu $ui
$backgroundLabels = Get-MenuItems $background
$backgroundLabels | ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $Evidence 'background-menu.json')
if (-not (($backgroundLabels -join "`n") -match 'Set folder policy')) { throw 'background menu lacks Set folder policy' }
Save-Screenshot 'background-menu.png'
$backgroundNeed = 'Set folder policy'
Invoke-MenuItem $background $backgroundNeed
Start-Sleep -Seconds 2
$proposals = Invoke-Cli $Obit @('proposals')
if ($proposals -notmatch [regex]::Escape($sample)) { throw 'background policy action did not report the real folder path' }
$window.Quit()
Start-Sleep -Seconds 1

# Verify the action path and label cleanup without relying on Explorer refresh timing.
$iniBefore = Get-Content -Raw (Join-Path $sample 'desktop.ini')
if ($iniBefore -notmatch 'alpha=alpha（\d\d-\d\d 下葬）') { throw 'Chinese date label was not written to desktop.ini' }
$alpha = Join-Path $sample 'alpha.txt'
$window = Find-Explorer $sample
$ui = Get-UiExplorer $window
Rename-ExplorerItem $ui 'alpha.txt' 'renamed.txt'
$window.Quit()
Start-Sleep -Seconds 1
$iniAfterRename = Get-Content -Raw (Join-Path $sample 'desktop.ini')
if ($iniAfterRename -notmatch 'alpha=alpha（' -or $iniAfterRename -match 'renamed=') { throw 'label changed with F2 rename; expected display-name mapping to preserve the real file name' }
Invoke-Cli $Obit @('config', '--language', 'en') | Out-Host
Invoke-Cli $Obit @('init', '--yes', '--no-shortcuts', '--no-skill') | Out-Host
$window = Find-Explorer $sample
$ui = Get-UiExplorer $window
Invoke-ExplorerAction $ui @('beta.txt') 'Bury now' 'bury-menu.json'
Assert-MenuResult 'bury'
$window.Quit()
Start-Sleep -Seconds 2
if (Test-Path (Join-Path $sample 'beta.txt')) { throw 'Bury now did not remove beta.txt from its original location' }
$window = Find-Explorer $sample
$ui = Get-UiExplorer $window
Invoke-ExplorerAction $ui @('foreign.txt') 'Resurrect' 'resurrect-menu.json'
Assert-MenuResult 'resurrect'
$window.Quit()
Start-Sleep -Seconds 2
if (-not (Test-Path (Join-Path $sample 'beta.txt'))) { throw 'Resurrect latest did not restore beta.txt' }
Invoke-Cli $Obit @('context-menu', 'policy', '--', $sample) | Out-Host
if (-not (Test-Path (Join-Path $sample 'desktop.ini'))) { throw 'desktop.ini disappeared after policy operation' }
if ((Get-Content -Raw (Join-Path $sample 'desktop.ini')) -notmatch 'foreign.txt=Keep this foreign label') { throw 'foreign desktop.ini entry was lost' }

Get-ChildItem $Evidence -File | Select-Object FullName, Length | Format-Table -AutoSize
Stop-Transcript
